/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	// "bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	// "path/filepath"

	"bytes"
	"encoding/json"
	"encoding/xml"

	"github.com/beevik/etree"
	"github.com/briandowns/spinner"
	"github.com/gosuri/uilive"
	"github.com/spf13/cobra"
)

type QueueInfo struct {
	Reason   string        `json:"why"`
	Location BuildLocation `json:"executable"`
}

type BuildLocation struct {
	Url string `json:"url"`
}

type JenkinsOutput struct {
	BuildUrl       string
	done           bool
	TerminalWriter uilive.Writer
	errorCount     int
}

var File string

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a Jenkins job with a new pipeline script",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		// Check if the file exists
		if _, err := os.Stat(File); os.IsNotExist(err) {
			log.Println("Error: File does not exist.")
			log.Fatal("Error: Could not read pipeline script from file", File)
		}
		config, _ := getJobConfig("foo")
		newPipeline, err := loadPipelineScriptFromFile(filepath.Clean(File))
		if err != nil {
			log.Println("Error:", err)
			log.Fatal("Error: Could not read pipeline script from file", File)
		}
		updatedScript, _ := replacePipelineScript(config, newPipeline)
		updateJobConfig("foo", updatedScript)
		log.Println("Info: Updated pipeline script for job foo")
		// Trigger a build
		buildUrl := triggerBuild("foo")
		// log.Println("Info: Build URL:", buildUrl)
		time.Sleep(1 * time.Second)
		// Stream the output of the build console to the terminal
		writer := uilive.New()
		jo := JenkinsOutput{BuildUrl: buildUrl, done: false, TerminalWriter: *writer, errorCount: 0}
		jo.streamBuildOutput(true)

	},
}

func (jo *JenkinsOutput) streamBuildOutput(filterOutput bool) {
	// Setup console output request
	req, err := http.NewRequest("GET", jo.BuildUrl+"/logText/progressiveText", nil)
	if err != nil {
		log.Println("Error:", err)
	}
	req.SetBasicAuth(User, APIKey)
	client := &http.Client{}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error:", err)
		log.Println("Error: Could not connect to Jenkins server. Please check the address and try again.")
	}
	defer resp.Body.Close()
	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error:", err)
		jo.done = true
	}
	// Write the response to the terminal, replacing the previous output
	jo.TerminalWriter.Start()
	if filterOutput {
		outbody := removePipelinePart(string(body))
		fmt.Fprintf(&jo.TerminalWriter, outbody)
	} else {
		fmt.Fprintf(&jo.TerminalWriter, string(body))

	}
	jo.TerminalWriter.Stop()
	// Check if the build is still running
	xMoreData := resp.Header.Get("X-More-Data")
	if xMoreData != "true" {
		jo.done = true
		return
	}

	if !jo.done {
		time.Sleep(5 * time.Second)
		jo.streamBuildOutput(filterOutput)
	}
	return

}

func removePipelinePart(consoleOutput string) string {
	// Remove all lines that contain `[Pipeline]` from the console output
	regexp := regexp.MustCompile(`(?m)\[Pipeline\].*\n`)
	res := regexp.ReplaceAllString(consoleOutput, "")
	return res
}

func triggerBuild(jobName string) string {
	// Set up the spinner
	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond) // Build our new spinner
	s.Color("yellow")
	s.Suffix = " Triggering build..."
	s.Start() // Start the spinner
	// Trigger the build
	jobUrl := Address + "/job/" + jobName + "/build?delay=0sec"
	req, _ := http.NewRequest("POST", jobUrl, nil)
	req.SetBasicAuth(User, APIKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error:", err)
		log.Println("Error: Could not trigger build for job", jobName)
	}
	defer resp.Body.Close()
	headers := resp.Header
	// Read queue location from headers
	queueLocation := headers.Get("Location")
	// Check if the build is in the queue
	// Loop until the build is no longer in the queue
	var buildUrl string
	var inQueue bool = true
	s.Suffix = " Waiting for build to start..."
	s.Color("green")
	for {
		buildUrl, inQueue = checkInQueue(queueLocation)
		if !inQueue {
			break
		}
		time.Sleep(1 * time.Second)
	}
	finalMsg := fmt.Sprintf("🚀 Build started successfully!\n🔗 Build url is: %sconsole\n🗎  Will now serve console output in real time\n\n", buildUrl)
	s.FinalMSG = finalMsg
	s.Stop()
	return buildUrl
}

func checkInQueue(queueLocation string) (string, bool) {
	queueUrl := queueLocation + "api/json"
	req, _ := http.NewRequest("GET", queueUrl, nil)
	req.SetBasicAuth(User, APIKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error:", err)
		log.Println("Error: Could not check build queue for queueLocation", queueLocation)
	}
	defer resp.Body.Close()
	// Get json output from the response
	var queueInfo QueueInfo
	json.NewDecoder(resp.Body).Decode(&queueInfo)
	// If the queueInfo.Reason is not empty, the build is still in the queue
	if queueInfo.Reason != "" {
		// log.Println("Info: Build is in queue. Reason:", queueInfo.Reason)
		return "", true
	}

	return queueInfo.Location.Url, false
}

func loadPipelineScriptFromFile(filename string) (string, error) {
	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()
	// Read the file
	script, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	// Return the Script
	return string(script), nil

}

// replacePipelineScript replaces the pipeline script in the config.xml with the newPipeline
func replacePipelineScript(config, newPipeline string) (string, error) {
	// Save the old xml header
	oldXMLHeader := config[:38]
	// Remove xml version 1.1 from config and insert version 1.0
	config = xml.Header + config[38:]
	// Parse the XML string
	doc := etree.NewDocument()
	if err := doc.ReadFromString(config); err != nil {
		log.Printf("Error reading XML: %v\n", err)
		return "", err
	}

	// Find the script element
	script := doc.FindElement("//script")
	// Set the text of the script element to the newPipeline
	script.SetText(newPipeline)
	updatedConfig, _ := doc.WriteToString()
	// Add the old xml header back to the updatedConfig
	updatedConfig = oldXMLHeader + updatedConfig[39:]
	log.Println(updatedConfig)
	return updatedConfig, nil
}

func updateJobConfig(jobName, updatedConfig string) error {
	// Make a get request to url at Address to check if Jenkins is alive
	jobUrl := Address + "/job/" + jobName + "/config.xml"
	req, err := http.NewRequest("POST", jobUrl, bytes.NewBuffer([]byte(updatedConfig)))
	if err != nil {
		log.Println("Error:", err)
	}
	req.SetBasicAuth(User, APIKey)
	// Set the content type to xml
	req.Header.Set("Content-Type", "text/xml")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error:", err)
		log.Println("Error: Could not connect to Jenkins server. Please check the address and try again.")
		return err
	}
	defer resp.Body.Close()

	return nil
}

func getJobConfig(jobName string) (string, error) {
	// Make a get request to url at Address to check if Jenkins is alive
	jobUrl := Address + "/job/" + jobName + "/config.xml"
	req, err := http.NewRequest("GET", jobUrl, nil)
	if err != nil {
		log.Println("Error:", err)
	}
	log.Println("User:", User)
	log.Println("APIKey:", APIKey)
	req.SetBasicAuth(User, APIKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error:", err)
		log.Println("Error: Could not connect to Jenkins server. Please check the address and try again.")
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error:", err)
		return "", err
	}
	return string(body), nil
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().StringVarP(&File, "file", "f", "", "Path to the pipeline script file.")
	updateCmd.MarkFlagRequired("file")
}
