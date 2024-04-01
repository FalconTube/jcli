package jenkins

import (
	"encoding/xml"
	"github.com/beevik/etree"
	"io"
	"log"
	"os"
	"regexp"
)

// ReplacePipelineScript replaces the pipeline script in the config.xml with the newPipeline
func ReplacePipelineScript(config, newPipeline string) (string, error) {
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
	// log.Println(updatedConfig)
	return updatedConfig, nil
}

// LoadPipelineScriptFromFile loads the pipeline script from a file
func LoadPipelineScriptFromFile(filename string) (string, error) {
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

// RemovePipelinePart removes the [Pipeline] part from the console output
func RemovePipelinePart(consoleOutput string) string {
	regexp := regexp.MustCompile(`(?m)\[Pipeline\].*\n`)
	res := regexp.ReplaceAllString(consoleOutput, "")
	return res
}
