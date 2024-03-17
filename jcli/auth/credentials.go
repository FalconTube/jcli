package auth

import (
	"log"

	"github.com/zalando/go-keyring"
)

func LoadAPIKeyfromKeyring(address, user string) string {
	// Load the API Key from the keyring
	serverAddress := "jcli::" + address
	apiKey, err := keyring.Get(serverAddress, user)
	if err != nil {
		if err.Error() == keyring.ErrNotFound.Error() {
			log.Printf("Error: No credentials found for server %s.\n", address)
			log.Fatal("Error: Use the 'auth' command to authenticate with the server.")
		}
		log.Println("Error:", err)
		log.Fatal("Error: Could not load API Key from keyring. Please install a keyring for your OS.")
	}
	return apiKey
}
