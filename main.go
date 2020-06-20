package main

import (
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/boromil/news/feed"
	"github.com/sirupsen/logrus"
)

var flagDir = flag.String("dir", "", "directory to store html files. By default ./news is used and created if necessary")
var flagTimeout = flag.Duration("timeout", 10*time.Second, "timeout in seconds when fetching feeds")
var flagUpdateInterval = flag.Duration("wait", 10*time.Minute, "minutes to wait between updates")
var flagItemsPerPage = flag.Int("items", 500, "number of items per page.html file. A new page.html file is created whenever index.html contains 2x that number")
var flagVerbose = flag.Bool("verbose", false, "verbose mode outputs extra info when enabled")
var flagTemplateFile = flag.String("template", "", "custom Go html/template file to use when generating .html files. See `news/feed/template.go`")
var flagOPMLFile = flag.String("opml", "", "path to OPML file containing feed URLS to be imported. Existing feed URLs are ovewritten, not duplicated")
var flagMinDomainRequestInterval = flag.Duration("noflood", 30*time.Second, "minium seconds between calls to same domain to avoid flooding")

func main() {
	flag.Parse()
	*flagTimeout = minMaxDuration(*flagTimeout, time.Second, time.Minute)
	*flagItemsPerPage = minMax(*flagItemsPerPage, 2, 500)
	*flagUpdateInterval = minMaxDuration(*flagUpdateInterval, 1, 30*time.Minute)
	*flagMinDomainRequestInterval = minMaxDuration(*flagMinDomainRequestInterval, 10, 24*time.Hour)

	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)
	if *flagVerbose {
		log.SetLevel(logrus.DebugLevel)
	}

	if *flagTemplateFile != "" {
		tpl, err := template.ParseFiles(*flagTemplateFile)
		if err != nil {
			log.Fatalf("Could not load custom template file: %s", err)
		}
		feed.Tpl = tpl
	}
	agg, err := feed.NewWithCustom(
		log,
		*flagDir,
		*flagItemsPerPage,
		feed.MakeURLFetcher(
			log,
			*flagMinDomainRequestInterval,
			&http.Client{Timeout: *flagTimeout},
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
	if *flagOPMLFile != "" {
		importedFeeds, err := agg.ImportOPMLFile(*flagOPMLFile)
		if err != nil {
			log.Fatalf("Could not import OPML file: %s", err)
		} else {
			log.Printf("Successfully imported %d feeds from OPML file.", importedFeeds)
		}
	}

	go func() {
		for {
			log.Infof("Fetching news from %d feed sources...", len(agg.Feeds))
			if err := agg.Update(); err != nil {
				log.Fatalln(err)
			}
			log.Infof("Done. Waiting %d minutes for next update...", *flagUpdateInterval)
			time.Sleep(*flagUpdateInterval)
		}
	}()

	pressCTRLCToExit()
	fmt.Println("Bye :)")
}

func pressCTRLCToExit() {
	exitCh := make(chan os.Signal)
	signalCh := make(chan os.Signal)
	signal.Notify(signalCh, os.Interrupt)
	go func() {
		exitCh <- (<-signalCh)
	}()
	<-exitCh
}

func minMaxDuration(value, min, max time.Duration) time.Duration {
	return time.Duration(minMax(int(value), int(min), int(max)))
}

func minMax(value, min, max int) int {
	if value < min {
		return min
	} else if value > max {
		return max
	}
	return value
}
