//go:build ignore

// tts_smoke.go — Phase 0 connectivity smoke test. Throwaway.
// Run once to confirm the API key + voice work end to end before building anything:
//
//	ELEVENLABS_API_KEY=sk_... ELEVENLABS_VOICE_ID=... go run backend/scripts/tts_smoke.go
//
// It writes smoke.mp3 in the current dir. Play it. If it plays, Phase 0 is done.
// NOTE: verify the endpoint/body against current ElevenLabs docs — its whole job
// is to fail loudly, so a wrong URL or param surfaces immediately.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	key := os.Getenv("ELEVENLABS_API_KEY")
	voice := os.Getenv("ELEVENLABS_VOICE_ID")
	if key == "" || voice == "" {
		fmt.Fprintln(os.Stderr, "set ELEVENLABS_API_KEY and ELEVENLABS_VOICE_ID")
		os.Exit(1)
	}

	body, _ := json.Marshal(map[string]any{
		"text":     "Ops, analytics. This is a control room voice check. Message rate nominal.",
		"model_id": "eleven_flash_v2_5",
	})

	url := "https://api.elevenlabs.io/v1/text-to-speech/" + voice
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("xi-api-key", key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/mpeg")

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "request failed:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "API %d: %s\n", resp.StatusCode, msg)
		os.Exit(1)
	}

	f, err := os.Create("smoke.mp3")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	n, _ := io.Copy(f, resp.Body)
	fmt.Printf("wrote smoke.mp3 (%d bytes) — play it\n", n)
}
