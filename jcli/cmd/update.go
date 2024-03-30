/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"bytes"
	"encoding/json"
	"encoding/xml"
	"strings"

	"jcli/auth"

	"github.com/beevik/etree"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	currentPkgNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("211"))
	doneStyle           = lipgloss.NewStyle().Margin(1, 1, 0)
	helpStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Margin(0, 1)
	checkMark           = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("✓")
)

type QueueInfo struct {
	Reason   string        `json:"why"`
	Location BuildLocation `json:"executable"`
}

type BuildLocation struct {
	Url string `json:"url"`
}

type BuildModel struct {
	BuildUrl      string
	File          string
	width         int
	height        int
	done          bool
	statusMessage string
	userScrolled  bool
	spinner       spinner.Model
	viewport      viewport.Model
}

type consoleOutput string
type consoleFinish string
type emptyUrl string

// var File string
var FullLog string

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a Jenkins job with a new pipeline script",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		main()
	},
}

func (m *BuildModel) initBuild() tea.Cmd {
	return func() tea.Msg {
		APIKey = auth.LoadAPIKeyfromKeyring(Address, User)

		// Check if the file exists
		if _, err := os.Stat(m.File); os.IsNotExist(err) {
			log.Println("Error: File does not exist.")
			log.Fatal("Error: Could not read pipeline script from file", m.File)
		}
		config, _ := getJobConfig("foo")
		newPipeline, err := loadPipelineScriptFromFile(filepath.Clean(m.File))
		if err != nil {
			log.Println("Error:", err)
			log.Fatal("Error: Could not read pipeline script from file", m.File)
		}
		updatedScript, _ := replacePipelineScript(config, newPipeline)
		updateJobConfig("foo", updatedScript)
		log.Println("Info: Updated pipeline script for job foo")
		// Trigger a build
		m.statusMessage = "💤 Waiting for job to start..."
		buildUrl := triggerBuild("foo")
		// log.Println("Info: Build URL:", buildUrl)
		time.Sleep(1 * time.Second)
		// Stream the output of the build console to the terminal
		log.Println("Info: Build URL inside initBuild:", buildUrl)
		m.BuildUrl = buildUrl
		m.statusMessage = " 👷 Executing build..."

		return consoleOutput("No console output yet...")
	}
}

func (m *BuildModel) GetBuildOutput(filterOutput bool) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(3 * time.Second)
		if m.BuildUrl == "" {
			return emptyUrl("No build URL found. Need to trigger build first.")
		}
		// Setup console output request
		req, err := http.NewRequest("GET", m.BuildUrl+"/logText/progressiveText", nil)
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
			m.done = true
		}
		// Check if the build is still running
		xMoreData := resp.Header.Get("X-More-Data")
		if xMoreData != "true" {
			m.done = true
			return consoleFinish("Build finished!")
			// return consoleOutput(string(body))
		}
		// Get the new log as raw
		rawLog := string(body)
		var cleanedLog string
		// Remove the [Pipeline] part from the console output
		if filterOutput {
			cleanedLog = removePipelinePart(rawLog)
		} else {
			cleanedLog = rawLog
		}

		// Check if the full log is empty
		var newLog string
		if FullLog == "" {
			log.Println("Setting full log to new log")
			FullLog = strings.Clone(cleanedLog)
			newLog = FullLog
		} else {
			// If the full log is not empty, get the new logText
			newLog = strings.ReplaceAll(cleanedLog, FullLog, "")
			FullLog = strings.Clone(cleanedLog)
		}
		_ = newLog
		return consoleOutput(FullLog)

	}
}

// removePipelinePart removes the [Pipeline] part from the console output
func removePipelinePart(consoleOutput string) string {
	regexp := regexp.MustCompile(`(?m)\[Pipeline\].*\n`)
	res := regexp.ReplaceAllString(consoleOutput, "")
	return res
}

func triggerBuild(jobName string) string {
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
	for {
		buildUrl, inQueue = checkInQueue(queueLocation)
		// Check not in queue and we have valid build url
		if !inQueue && buildUrl != "" {
			break
		}
		time.Sleep(1 * time.Second)
	}
	// fmt.Printf("\n🚀 Build started successfully!\n🔗 Build url is: %sconsole\n🗎  Will now serve console output in real time\n\n", buildUrl)
	// finalMsg := fmt.Sprintf("🚀 Build started successfully!\n🔗 Build url is: %sconsole\n🗎  Will now serve console output in real time\n\n", buildUrl)
	return buildUrl
}

// checkInQueue checks if the build is still in the queue
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

// loadPipelineScriptFromFile loads the pipeline script from a file
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
	// log.Println(updatedConfig)
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

func init() {
	rootCmd.AddCommand(updateCmd)
	// updateCmd.Flags().StringVarP(&File, "file", "f", "", "Path to the pipeline script file.")
	// updateCmd.MarkFlagRequired("file")
}

func NewBuildModel(filename string, width int, height int) *BuildModel {
	// Setup viewport initial dimensions. Will be set to full screen size in the first update.
	vp := viewport.New(width-3, height-7)
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		Margin(1, 1, 0)

	vp.SetContent("No console output yet...")

	// Setup spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	return &BuildModel{
		File:          filename,
		spinner:       s,
		viewport:      vp,
		userScrolled:  false,
		statusMessage: "⌚ Triggering job ...",
	}
}

func (m *BuildModel) Init() tea.Cmd {
	log.Println("Init in BuildModel")
	return tea.Batch(m.GetBuildOutput(true), m.spinner.Tick)
}

func (m *BuildModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		log.Println("Window size message")
		m.width, m.height = msg.Width, msg.Height
		m.viewport.Height = m.height - 7
		m.viewport.Width = m.width - 3
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			return m, tea.Quit
		case "k", "up", "j", "down", "home", "end":
			m.userScrolled = true
		case "ctrl+u", "pageup":
			m.userScrolled = true
			m.viewport.ViewUp()
		case "ctrl+d", "pagedown":
			m.userScrolled = true
			m.viewport.ViewDown()
		case "a", "G":
			// Append to auto scroll again
			m.userScrolled = false
			m.viewport.GotoBottom()
		}
	case emptyUrl:
		log.Println("Empty URL")
		cmds = append(cmds, m.initBuild())
	case consoleFinish:
		// Build finished
		m.statusMessage = checkMark.Render() + " Build finished!"
		m.done = true
		// return m, tea.Quit
	case consoleOutput:
		m.viewport.SetContent(string(msg))
		// If user scrolled manually, don't auto-scroll
		if !m.userScrolled {
			m.viewport.GotoBottom()
		}
		// m.viewport, cmd = m.viewport.Update(msg)

		cmds = append(cmds, m.GetBuildOutput(true))
	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m BuildModel) View() string {
	help := helpStyle.Render(fmt.Sprintf("\n a/G: auto-scroll • j/↓: down • k/↑: up\n c+u/p-up: page up • c+d/p-down: page down •q: exit\n"))
	if m.done {
		return doneStyle.Render(m.statusMessage+"\n") + m.viewport.View() + help
	}
	spin := m.spinner.View() + " " + m.statusMessage + "\n"
	return doneStyle.Render(spin) + m.viewport.View() + help
}

func main() {
	if len(os.Getenv("DEBUG")) > 0 {
		f, err := tea.LogToFile("debug.log", "console")
		if err != nil {
			fmt.Println("fatal:", err)
			os.Exit(1)
		}
		defer f.Close()
	}
	testFile := "Jenkinsfile"
	width, height := 80, 24
	if _, err := tea.NewProgram(NewBuildModel(testFile, width, height), tea.WithMouseCellMotion(), tea.WithAltScreen()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
