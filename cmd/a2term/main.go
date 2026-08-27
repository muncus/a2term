// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"a2term/pkg/a2a"
	"a2term/pkg/ui"
)

var (
	version = "0.1.0"
)

func main() {
	agentFlag := flag.String("agent", "", "URL of the A2A agent endpoint (e.g. http://localhost:8080)")
	cardFlag := flag.String("card", "", "URL of the A2A Agent Card (e.g. http://localhost:8080/.well-known/agent-card.json)")
	flag.StringVar(agentFlag, "a", "", "Short for --agent")
	flag.StringVar(cardFlag, "c", "", "Short for --card")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	flag.BoolVar(versionFlag, "v", false, "Short for --version")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "a2term - Terminal Client for A2A Agents with A2UI Rendering\n\n")
		fmt.Fprintf(os.Stderr, "Usage: a2term [options]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment Variables:\n")
		fmt.Fprintf(os.Stderr, "  A2A_AGENT_URL    Default agent endpoint URL\n")
		fmt.Fprintf(os.Stderr, "  A2A_CARD_URL     Default agent card URL\n")
	}

	flag.Parse()

	if *versionFlag {
		fmt.Printf("a2term v%s\n", version)
		os.Exit(0)
	}

	agentURL := *agentFlag
	if agentURL == "" {
		agentURL = os.Getenv("A2A_AGENT_URL")
	}

	cardURL := *cardFlag
	if cardURL == "" {
		cardURL = os.Getenv("A2A_CARD_URL")
	}

	var client *a2a.Client
	var initErr error
	if agentURL != "" || cardURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cli, err := a2a.NewClient(ctx, a2a.ClientOptions{
			AgentURL: agentURL,
			CardURL:  cardURL,
		})
		cancel()
		if err != nil {
			initErr = err
		} else {
			client = cli
		}
	}

	cfg := ui.Config{
		Client:     client,
		AgentURL:   agentURL,
		CardURL:    cardURL,
		InitialErr: initErr,
	}

	model := ui.NewModel(cfg)
	p := tea.NewProgram(model)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running a2term: %v\n", err)
		os.Exit(1)
	}
}
