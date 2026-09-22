package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/opencode-ai/dhriti/internal/auth"
	"github.com/opencode-ai/dhriti/internal/config"
	"github.com/spf13/cobra"
)

const (
	// Google OAuth app credentials for Dhriti (installed / loopback client).
	// Override with DHRITI_GOOGLE_CLIENT_ID / DHRITI_GOOGLE_CLIENT_SECRET.
	defaultGoogleClientID     = ""
	defaultGoogleClientSecret = ""

	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
	geminiScope    = "https://www.googleapis.com/auth/generative-language"

	openaiAPIKeysURL = "https://platform.openai.com/api-keys"
	aistudioKeyURL   = "https://aistudio.google.com/apikey"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in with ChatGPT (OpenAI) and Gemini for gateway WebSocket use",
	Long: `Opens your browser to authenticate, then stores credentials in ~/.dhriti.json.

  • ChatGPT / OpenAI — opens the API keys page; paste the key when prompted.
  • Gemini — Google OAuth (loopback) when a client ID is configured, otherwise
    opens Google AI Studio to create an API key.
  • Gateway — optional WebSocket gateway URL used for streaming.

When gateway.url is set, Dhriti routes LLM traffic through the gateway and
uses the stored OpenAI key as the Bearer token if gateway.apiKey is empty.`,
	Example: `
  # Interactive login (OpenAI + Gemini + optional gateway)
  dhriti login

  # Only configure the WebSocket gateway
  dhriti login --gateway-only

  # Skip browser; paste keys only
  dhriti login --no-browser
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		gatewayOnly, _ := cmd.Flags().GetBool("gateway-only")
		noBrowser, _ := cmd.Flags().GetBool("no-browser")

		// Config must be loaded so updateCfgFile can resolve paths.
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		if _, err := config.Load(cwd, false); err != nil {
			return err
		}

		in := bufio.NewReader(os.Stdin)

		if !gatewayOnly {
			if err := loginOpenAI(in, !noBrowser); err != nil {
				return err
			}
			if err := loginGemini(in, !noBrowser); err != nil {
				return err
			}
		}

		if err := loginGateway(in); err != nil {
			return err
		}

		fmt.Println("\nLogin complete. Credentials saved to config.")
		return nil
	},
}

func init() {
	loginCmd.Flags().Bool("gateway-only", false, "Only configure the WebSocket gateway URL/token")
	loginCmd.Flags().Bool("no-browser", false, "Do not open a browser; prompt for pasted keys only")
	rootCmd.AddCommand(loginCmd)
}

func prompt(in *bufio.Reader, label string) (string, error) {
	fmt.Printf("%s: ", label)
	line, err := in.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func loginOpenAI(in *bufio.Reader, openBrowser bool) error {
	fmt.Println("\n── ChatGPT / OpenAI ──")
	if openBrowser {
		_ = auth.OpenBrowser(openaiAPIKeysURL)
		fmt.Println("Opened", openaiAPIKeysURL)
	}
	fmt.Println("Create or copy an API key, then paste it below.")
	key, err := prompt(in, "OpenAI API key (empty to skip)")
	if err != nil {
		return err
	}
	if key == "" {
		fmt.Println("Skipped OpenAI.")
		return nil
	}
	return config.SaveProviderKey("openai", key)
}

func loginGemini(in *bufio.Reader, openBrowser bool) error {
	fmt.Println("\n── Gemini ──")

	clientID := os.Getenv("DHRITI_GOOGLE_CLIENT_ID")
	if clientID == "" {
		clientID = defaultGoogleClientID
	}
	clientSecret := os.Getenv("DHRITI_GOOGLE_CLIENT_SECRET")
	if clientSecret == "" {
		clientSecret = defaultGoogleClientSecret
	}

	if clientID != "" {
		fmt.Println("Starting Google OAuth in your browser…")
		tok, err := auth.AuthCodeFlow(context.Background(), auth.OAuthConfig{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			AuthURL:      googleAuthURL,
			TokenURL:     googleTokenURL,
			Scopes:       []string{geminiScope},
		}, 5*time.Minute)
		if err != nil {
			return fmt.Errorf("gemini oauth: %w", err)
		}
		// Store the access token as the provider credential (Bearer for Live/REST).
		if err := config.SaveProviderKey("gemini", tok.AccessToken); err != nil {
			return err
		}
		if tok.RefreshToken != "" {
			_ = config.SaveGeminiRefreshToken(tok.RefreshToken)
		}
		fmt.Println("Gemini signed in via Google OAuth.")
		return nil
	}

	// Fallback: AI Studio API key (browser-assisted).
	if openBrowser {
		_ = auth.OpenBrowser(aistudioKeyURL)
		fmt.Println("Opened", aistudioKeyURL)
	}
	fmt.Println("Create a Gemini API key in AI Studio, then paste it below.")
	key, err := prompt(in, "Gemini API key (empty to skip)")
	if err != nil {
		return err
	}
	if key == "" {
		fmt.Println("Skipped Gemini.")
		return nil
	}
	return config.SaveProviderKey("gemini", key)
}

func loginGateway(in *bufio.Reader) error {
	fmt.Println("\n── WebSocket gateway (optional) ──")
	url, err := prompt(in, "Gateway URL wss://… (empty to skip)")
	if err != nil {
		return err
	}
	if url == "" {
		fmt.Println("Skipped gateway.")
		return nil
	}
	key, err := prompt(in, "Gateway API key (empty = use OpenAI key from login)")
	if err != nil {
		return err
	}
	return config.SaveGateway(url, key)
}
