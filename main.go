package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jtagcat/util"
	"k8s.io/apimachinery/pkg/util/wait"
)

func main() {
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt)

	dir, ok := os.LookupEnv("OUTDIR")
	if !ok {
		log.Fatal("OUTDIR not set")
	}

	urlStr, ok := os.LookupEnv("URL")
	if !ok {
		log.Fatal("URL not set")
	}

	period, err := time.ParseDuration(os.Getenv("PERIOD"))
	if err != nil {
		log.Fatalf("parsing PERIOD: %e", err)
	}

	jitterStr, ok := os.LookupEnv("JITTER")
	if !ok {
		jitterStr = "0.2"
	}
	jitter, err := strconv.ParseFloat(jitterStr, 64)
	if err != nil {
		log.Fatalf("parsing JITTER: %e", err)
	}

	rolling := util.NewRollingCsvAppender(func() string {
		return filepath.Join(dir, time.Now().UTC().Format("2006-01-02")+".csv")
	}, 0o660)
	defer rolling.Close()

	if err := rolling.WriteCurrent([]string{
		time.Now().UTC().Format(time.RFC3339),
		fmt.Sprintf("#LOG: started url-tocsv for %s", urlStr),
	}); err != nil {
		log.Fatalf("writing startup string: %e", err)
	}

	var r routine
	wait.JitterUntilWithContext(ctx, func(ctx context.Context) {
		w, err, _ := rolling.Current()
		if err != nil {
			log.Fatalf("opening current file: %e", err)
		}

		if err := r.run(ctx, w, urlStr); err != nil {
			log.Printf("routine unsuccessful: %e", err)
			_ = w.Write([]string{
				time.Now().UTC().Format(time.RFC3339),
				fmt.Sprintf("#LOG: failed getting %s", urlStr),
			})
		}
	},
		period, jitter, false)
}

type routine struct {
	lastSet bool
	last    []byte
}

func (r *routine) run(ctx context.Context, appender *csv.Writer, urlStr string) error {
	res, err := http.Get(urlStr)
	if err != nil {
		return fmt.Errorf("request to url: %w", err)
	}
	if res.StatusCode > 299 {
		return fmt.Errorf("response has non-ok status: %s", res.Status)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if r.lastSet && bytes.Equal(body, r.last) {
		return nil
	}
	r.lastSet = true
	r.last = body

	err = appender.Write([]string{
		time.Now().UTC().Format(time.RFC3339),
		string(body),
	})

	appender.Flush()
	return err
}
