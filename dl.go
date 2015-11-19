package main

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"os"

	"github.com/itchio/butler/bio"
)

const bufferSize = 128 * 1024

func dl() {
	if len(os.Args) < 4 {
		bio.Die("Missing url or dest for dl command")
	}
	url := os.Args[2]
	dest := os.Args[3]

	tries := 3
	for tries > 0 {
		_, err := tryDl(url, dest)
		if err == nil {
			break
		}

		bio.Log(fmt.Sprintf("While downloading, got error %s", err))
		tries--
		if tries > 0 {
			os.Truncate(dest, 0)
			bio.Log(fmt.Sprintf("Retrying... (%d tries left)", tries))
		} else {
			bio.Die(err.Error())
		}
	}
}

func tryDl(url string, dest string) (int64, error) {
	existingBytes := int64(0)
	stats, err := os.Lstat(dest)
	if err == nil {
		existingBytes = stats.Size()
	}

	bio.Log(fmt.Sprintf("existing file is %d bytes long", existingBytes))

	client := &http.Client{}

	req, _ := http.NewRequest("GET", url, nil)
	byteRange := fmt.Sprintf("bytes=%d-", existingBytes)
	bio.Logf("Asking for range %s", byteRange)

	req.Header.Set("Range", byteRange)
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	doDownload := true
	totalBytes := existingBytes + resp.ContentLength

	switch resp.StatusCode {
	case 200: // OK
		bio.Logf("server 200'd, does not support byte ranges")
		// will send data, but doesn't support byte ranges
		existingBytes = 0
		totalBytes = resp.ContentLength
		os.Truncate(dest, 0)
	case 206: // Partial Content
		bio.Logf("server 206'd, supports byte ranges")
		// will send incremental data
	case 416: // Requested Range not Satisfiable
		bio.Logf("server 416'd")
		// already has everything
		doDownload = false

		req, _ := http.NewRequest("HEAD", url, nil)
		resp, err = client.Do(req)
		if err != nil {
			return 0, err
		}

		if existingBytes > resp.ContentLength {
			bio.Logf("existing file too big (%d), truncating to %d", existingBytes, resp.ContentLength)
			existingBytes = resp.ContentLength
			os.Truncate(dest, existingBytes)
		}
		totalBytes = existingBytes
	default:
		return 0, fmt.Errorf("server error: http %s", resp.Status)
	}

	if doDownload {
		bio.Log(fmt.Sprintf("Response content length = %d", resp.ContentLength))
		err := appendAllToFile(resp.Body, dest, existingBytes, totalBytes)
		if err != nil {
			return 0, err
		}
		bio.Log("done downloading")
	} else {
		bio.Log("all downloaded already")
	}

	_, err = checkIntegrity(resp, totalBytes, dest)
	if err != nil {
		return 0, err
	}

	return totalBytes, nil
}

func appendAllToFile(src io.Reader, dest string, existingBytes int64, totalBytes int64) (err error) {
	out, _ := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	defer out.Close()

	prevPercent := 0.0

	onWrite := func(bytesDownloaded int64) {
		bytesWritten := existingBytes + bytesDownloaded
		percent := float64(bytesWritten) * 100.0 / float64(totalBytes)
		if math.Abs(percent-prevPercent) < 0.1 {
			return
		}

		prevPercent = percent
		bio.Progress(percent)
	}
	counter := bio.ProgressCounter(onWrite, out)

	_, err = io.Copy(counter, src)
	return
}
