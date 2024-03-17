package cmd

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"

	"net/http"

	"github.com/beevik/etree"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var Address string
var User string
var APIKey string

// connectCmd represents the connect command
var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect a Jenkins server",
	Long:  `Specify address of a Jenkins server to connect to.`,
	Run: func(cmd *cobra.Command, args []string) {
		err := godotenv.Load()
		if err != nil {
			log.Fatal("Error loading .env file")
		}
		APIKey = os.Getenv("API_KEY")

		// Make a get request to url at Address to check if Jenkins is alive
		checkServerRunning()
		log.Println("Info: Jenkins server is alive at", Address)
		hasAccess := checkAccessPermission()
		log.Println("Info: User has access:", hasAccess)
		config, _ := getJobConfig("foo")
		newPipeline, _ := loadPipelineScriptFromFile("mypipe.groovy")
		updatedScript, err := replacePipelineScript(config, newPipeline)
		updateJobConfig("foo", updatedScript)

	},
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
	oldXMLHeader := config[:39]
	// Remove xml version 1.1 from config and insert version 1.0
	config = xml.Header + config[39:]
	// Parse the XML string
	doc := etree.NewDocument()
	if err := doc.ReadFromString(config); err != nil {
		fmt.Printf("Error reading XML: %v\n", err)
		return "", err
	}

	// Find the script element
	script := doc.FindElement("//script")
	// Set the text of the script element to the newPipeline
	script.SetText(newPipeline)
	updatedConfig, _ := doc.WriteToString()
	// Add the old xml header back to the updatedConfig
	updatedConfig = oldXMLHeader + updatedConfig[39:]
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

// checkServerRunning checks if the Jenkins server is running and will exit if not
func checkServerRunning() {
	// Make a get request to url at Address to check if Jenkins is alive
	resp, err := http.Get(Address)
	if err != nil {
		log.Println("Error:", err)
		log.Fatal("Error: Could not connect to Jenkins server. Please check the address and try again.")
	}
	defer resp.Body.Close()
}

// checkAccessPermission checks if the user has access to the Jenkins server
func checkAccessPermission() bool {
	// Make a get request to url at Address including User and API Key to check if Jenkins is alive
	req, err := http.NewRequest("GET", Address, nil)
	if err != nil {
		log.Println("Error:", err)
		return false
	}
	req.SetBasicAuth(User, APIKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error:", err)
		return false
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case 403, 401:
		log.Fatal("Error: Access denied. Please check your credentials and try again.")
		return false
	default:
		log.Println("Info: Access granted to Jenkins at address:", Address)
	}
	io.ReadAll(resp.Body)
	return true

}

func init() {
	rootCmd.AddCommand(connectCmd)

	connectCmd.Flags().StringVarP(&Address, "address", "a", "", "Address of Jenkins server.")
	connectCmd.MarkFlagRequired("address")
	connectCmd.Flags().StringVarP(&User, "user", "u", "", "User to connect to Jenkins server.")
	connectCmd.MarkFlagRequired("user")
}
