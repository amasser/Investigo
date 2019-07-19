package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
)

import (
	color "github.com/fatih/color"
)

const (
	dataFileName string = "data.json"
	userAgent    string = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3770.100 Safari/537.36"
)

// Result of Investigo function
type Result struct {
	exist   bool
	link    string
	message string
}

var (
	wg       = &sync.WaitGroup{}
	logger   = log.New(color.Output, "", 0)
	notExist = Result{exist: false, message: color.HiYellowString("Not Found!")}
	options  struct {
		color           bool
		updateBeforeRun bool
		verbose         bool
	}
)

// A SiteData struct for json datatype
type SiteData struct {
	ErrorType string `json:"errorType"`
	ErrorMsg  string `json:"errorMsg"`
	URL       string `json:"url"`
	URLMain   string `json:"urlMain"`
	URLProbe  string `json:"urlProbe"`
	// UsedUsername   string `json:"username_claimed"`
	// UnusedUsername string `json:"username_unclaimed"`
	// RegexCheck string `json:"regexCheck"`
	// Rank int`json:"rank"`
}

var siteData = map[string]SiteData{}

// RequestError interface
type RequestError interface {
	Error() string
}

func initializeSiteData() {
	jsonFile, err := os.Open(dataFileName)
	if err != nil {
		fmt.Fprintf(
			color.Output,
			"%s Failed to read %s from current directory. %s",
			color.HiRedString("->"),
			dataFileName,
			color.HiYellowString("Downloading..."),
		)
		r, err := Request("https://raw.githubusercontent.com/tdh8316/Investigo/master/data.json")
		defer r.Body.Close()
		if err != nil || r.StatusCode != 200 {
			fmt.Fprintf(color.Output, " [%s]\n", color.HiRedString("Failed"))
			panic("Failed to connect to Investigo repository.")
		}
		if _, err := os.Stat(dataFileName); !os.IsNotExist(err) {
			if err = os.Remove(dataFileName); err != nil {
				panic(err)
			}
		}
		_updateFile, _ := os.OpenFile(dataFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if _, err := _updateFile.WriteString(ReadResponseBody(r)); err != nil {
			fmt.Fprintf(color.Output, color.RedString("Failed to update data\n"))
		}
		_updateFile.Close()
		jsonFile, _ = os.Open(dataFileName)
		
		fmt.Println(" [Done]")
	}

	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		panic("Error while read " + dataFileName)
	} else {
		json.Unmarshal([]byte(byteValue), &siteData)
	}
	return
}

func main() {
	fmt.Println(`Investigo - Investigate User Across Social Networks.`)

	args := os.Args[1:]
	var argIndex int

	options.color, argIndex = HasElement(args, "--no-color")
	if options.color {
		args = append(args[:argIndex], args[argIndex+1:]...)
	}

	options.verbose, argIndex = HasElement(args, "-v", "--verbose")
	if options.verbose {
		args = append(args[:argIndex], args[argIndex+1:]...)
	}

	if help, _ := HasElement(args, "-h", "--help"); help || len(args) < 1 {
		os.Exit(0)
	}

	// Loads site data from sherlock database and assign to a variable.
	initializeSiteData()

	for _, username := range args {
		wg.Add(len(siteData))
		for site := range siteData {
			go func(site string) {
				investigo := Investigo(username, site, siteData[site])
				if investigo.exist {
					WriteResult(site, true, investigo.link)
				} else {
					WriteResult(site, false, color.HiMagentaString(investigo.message))
				}
				wg.Done()
				runtime.Gosched()
				return
			}(site)
		}
		wg.Wait()
	}

	return
}

// Request makes HTTP request
func Request(url string) (*http.Response, RequestError) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", userAgent)
	client := &http.Client{}
	response, clientError := client.Do(req)

	return response, clientError
}

// ReadResponseBody reads response body and return string
func ReadResponseBody(response *http.Response) string {
	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		panic(err)
	}
	return string(bodyBytes)
}

// HasElement reports whether elements is within array.
func HasElement(array []string, targets ...string) (bool, int) {
	for index, item := range array {
		for _, target := range targets {
			if item == target {
				return true, index
			}
		}
	}
	return false, -1
}

// Investigo investigate if username exists on social media.
func Investigo(username string, site string, data SiteData) Result {
	var url, urlProbe string

	// string to display
	url = strings.Replace(data.URL, "{}", username, 1)

	if data.URLProbe != "" {
		urlProbe = strings.Replace(data.URLProbe, "{}", username, 1)
	} else {
		urlProbe = strings.Replace(data.URL, "{}", username, 1)
	}

	r, err := Request(urlProbe)
	defer r.Body.Close()
	if err != nil {
		if !options.verbose {
			logger.Printf(
				"[%s] %s: %s\n",
				color.RedString("!"), color.HiWhiteString(site), err.Error(),
			)
		}
		return Result{exist: false, message: err.Error()}
	}

	switch data.ErrorType {
	case "status_code":
		if r.StatusCode <= 300 || r.StatusCode < 200 {
			return Result{
				exist: true, link: url,
			}
		}
		return notExist
	case "message":
		if !strings.Contains(ReadResponseBody(r), data.ErrorMsg) {
			return Result{
				exist: true, link: url,
			}
		}
		return notExist
	case "response_url":
		if r.Request.URL.String() == url && (r.StatusCode <= 300 || r.StatusCode < 200) {
			return Result{
				exist: true, link: url,
			}
		}
		return notExist
	default:
		return Result{
			exist: false, message: "ERROR: Unsupported error type",
		}
	}
}

// WriteResult writes investigation result to stdout and file
func WriteResult(site string, exist bool, detail string) {
	if exist {
		logger.Printf(
			"[%s] %s: %s\n",
			color.HiGreenString("+"), color.HiWhiteString(site), detail,
		)
	} else {
		if options.verbose {
			logger.Printf(
				"[%s] %s: %s\n",
				color.HiRedString("-"), color.HiWhiteString(site), detail,
			)
		}
	}
	return
}
