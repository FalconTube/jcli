package jenkins

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

type QueueInfo struct {
	Reason   string        `json:"why"`
	Location BuildLocation `json:"executable"`
}

type BuildLocation struct {
	Url string `json:"url"`
}

type Jenkins struct {
	Address string
	User    string
	APIKey  string
}

type RequestError struct {
	StatusCode int
	Msg        string
}

func NewJenkins(address, user, apiKey string) *Jenkins {
	// Load the API Key from the keyring
	return &Jenkins{
		Address: address,
		User:    user,
		APIKey:  apiKey,
	}
}

func (j *Jenkins) UpdateJobConfig(jobName, updatedConfig string) error {
	// Make a get request to url at Address to check if Jenkins is alive
	jobUrl := j.Address + "/job/" + jobName + "/config.xml"
	req, err := http.NewRequest("POST", jobUrl, bytes.NewBuffer([]byte(updatedConfig)))
	if err != nil {
		return err
	}
	req.SetBasicAuth(j.User, j.APIKey)
	// Set the content type to xml
	req.Header.Set("Content-Type", "text/xml")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (j *Jenkins) CheckJobsExist(jobName string) bool {
	// Make a get request to url at Address to check if Jenkins is alive
	jobUrl := j.Address + "/job/" + jobName + "/config.xml"
	req, err := http.NewRequest("GET", jobUrl, nil)
	if err != nil {
		log.Println("Error:", err)
	}
	req.SetBasicAuth(j.User, j.APIKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error:", err)
		log.Println("Error: Could not connect to Jenkins server. Please check the address and try again.")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return false
	}
	return true
}

// CreateEmptyJob creates a new job with the given name
func (j *Jenkins) CreateEmptyJob(jobName string) error {
	// Read the standard config.xml file
	data, err := os.ReadFile("config.xml")
	if err != nil {
		log.Println("Error:", err)
	}

	// Setup the request
	jobUrl := j.Address + "/createItem"
	params := url.Values{}
	params.Add("name", jobName)
	params.Add("mode", "create")
	params.Add("Content-Type", "text/xml")

	req, _ := http.NewRequest("POST", jobUrl, bytes.NewBuffer(data))
	// Add the parameters to the request
	req.URL.RawQuery = params.Encode()
	for key, value := range params {
		req.Header.Add(key, value[0])
	}
	req.SetBasicAuth(j.User, j.APIKey)

	// Perform the request
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

func (j *Jenkins) GetJobConfig(jobName string) (string, error) {
	// Make a get request to url at Address to check if Jenkins is alive
	log.Println("Getting job config for", jobName)
	jobUrl := j.Address + "/job/" + jobName + "/config.xml"
	log.Println("jobURL", jobUrl)
	req, err := http.NewRequest("GET", jobUrl, nil)
	if err != nil {
		log.Println("Error:", err)
	}
	req.SetBasicAuth(j.User, j.APIKey)
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

// checkInQueue checks if the build is still in the queue
func (j *Jenkins) CheckInQueue(queueLocation string) (string, bool) {
	queueUrl := queueLocation + "api/json"
	req, _ := http.NewRequest("GET", queueUrl, nil)
	req.SetBasicAuth(j.User, j.APIKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error:", err)
		log.Fatal("Error: Could not check build queue for queueLocation", queueLocation)
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

func (j *Jenkins) TriggerBuild(jobName string) (string, error) {
	// Check if Jenkins job is parameterized
	isParametrized, err := j.JobIsParametrized(jobName)
	if err != nil {
		return "", err
	}
	if isParametrized {
		return "", errors.New("Error: Jenkins job is parameterized. Please run the job manually.")
	}
	// Trigger the build
	jobUrl := j.Address + "/job/" + jobName + "/build?delay=0sec"
	req, _ := http.NewRequest("POST", jobUrl, nil)
	req.SetBasicAuth(j.User, j.APIKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 201 {
		return "", errors.New("Error: Could not trigger build for job " + jobName)
	}
	defer resp.Body.Close()
	headers := resp.Header
	// Read queue location from headers
	queueLocation := headers.Get("Location")

	log.Println("Info: Build is in queue. Queue location:", queueLocation)
	// Check if the build is in the queue
	// Loop until the build is no longer in the queue
	var buildUrl string
	var inQueue bool = true
	for {
		buildUrl, inQueue = j.CheckInQueue(queueLocation)
		// Check not in queue and we have valid build url
		if !inQueue && buildUrl != "" {
			break
		}
		time.Sleep(1 * time.Second)
	}
	// fmt.Printf("\n🚀 Build started successfully!\n🔗 Build url is: %sconsole\n🗎  Will now serve console output in real time\n\n", buildUrl)
	// finalMsg := fmt.Sprintf("🚀 Build started successfully!\n🔗 Build url is: %sconsole\n🗎  Will now serve console output in real time\n\n", buildUrl)
	return buildUrl, nil
}

func (j *Jenkins) JobIsParametrized(jobName string) (bool, error) {
	// Check if Jenkins job is parameterized
	testUrl := j.Address + "/job/" + jobName + "/api/json"
	req, _ := http.NewRequest("GET", testUrl, nil)
	req.SetBasicAuth(j.User, j.APIKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, errors.New("Error: Could not connect to Jenkins server. Please check the address and try again.")
	}

	defer resp.Body.Close()
	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, errors.New("Error: Could not read response from Jenkins server.")
	}
	// Check if there are parameters
	var jobInfo map[string]interface{}
	json.Unmarshal(body, &jobInfo)
	if jobInfo["property"] != nil {
		properties := jobInfo["property"].([]interface{})
		for _, property := range properties {
			if property != nil {
				parameters := property.(map[string]interface{})["parameterDefinitions"]
				if parameters != nil {
					return true, nil
				}
			}
		}
	}
	return false, nil

}
