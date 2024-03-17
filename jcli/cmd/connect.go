package cmd

import (
	"io"
	"log"

	"net/http"

	"github.com/spf13/cobra"
)

// connectCmd represents the connect command
var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect a Jenkins server",
	Long:  `Specify address of a Jenkins server to connect to.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Make a get request to url at Address to check if Jenkins is alive
		checkServerRunning()
		log.Println("Info: Jenkins server is alive at", Address)
		hasAccess := checkAccessPermission()
		log.Println("Info: User has access:", hasAccess)

	},
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

	// connectCmd.Flags().StringVarP(&Address, "address", "a", "", "Address of Jenkins server.")
	// connectCmd.MarkFlagRequired("address")
	// connectCmd.Flags().StringVarP(&User, "user", "u", "", "User to connect to Jenkins server.")
	// connectCmd.MarkFlagRequired("user")
}
