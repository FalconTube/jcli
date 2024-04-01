package cmd

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"net/http"

	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
)

var (
	Token string
)

// connectCmd represents the connect command
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticates with a Jenkins server",
	Long:  `Specify address of a Jenkins server to connect to.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Make a get request to url at Address to check if Jenkins is alive
		log.Println("Jenkins", Jenkins)
		checkServerRunning()
		log.Println("Info: Jenkins server is alive at", Address)
		hasAccess := checkAccessPermission()
		log.Println("Info: User has access:", hasAccess)
		saveCredentialsToKeyring()

	},
}

// saveCredentialsToKeyring saves the user's credentials to the keyring
// so that they can be used in future sessions.
// Saves a keyring entry with the server address as the service name
func saveCredentialsToKeyring() {
	// Check if credentials are already saved
	serverAddress := "jcli::" + Address
	_, err := keyring.Get(serverAddress, User)
	if err != nil {
		if err.Error() == keyring.ErrNotFound.Error() {
			log.Println("Info: Credentials not yet saved to keyring. Saving now.")
			keyring.Set(serverAddress, User, Token)
			log.Println("Info: Credentials saved to keyring.")
			return
		}
		log.Println("Error:", err)
		log.Fatal("Error: Keyring not supported on this system. Install a keyring manager for your OS.", err)
	}
	if confirmInput(serverAddress) {
		keyring.Set(serverAddress, User, Token)
	}

}

func confirmInput(serverAddress string) bool {
	// Ask user if they want to overwrite the existing credentials
	tries := 3
	r := bufio.NewReader(os.Stdin)
	for ; tries > 0; tries-- {
		fmt.Printf("You are already logged into server %s. Do you want to update the credentials? [y/n]: ", serverAddress)

		res, err := r.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}

		// Empty input (i.e. "\n")
		if len(res) < 2 {
			continue
		}

		return strings.ToLower(strings.TrimSpace(res))[0] == 'y'
	}
	return false
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
	req.SetBasicAuth(User, Token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error:", err)
		return false
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case 403, 401:
		body, _ := io.ReadAll(resp.Body)
		log.Println(string(body))
		log.Fatal("Error: Access denied. Please check your credentials and try again.")
		return false
	default:
		log.Println("Info: Access granted to Jenkins at address:", Address)
	}
	io.ReadAll(resp.Body)
	return true

}

func init() {
	rootCmd.AddCommand(authCmd)

	authCmd.Flags().StringVarP(&Token, "token", "t", "", "API Token to connect to Jenkins server.")
	authCmd.MarkFlagRequired("token")
}
