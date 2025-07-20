package main

import (
	"flag"
	"fmt"
	"os"
)

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  server --file   # Serve feeds from a directory structure (filesystem mode)")
	fmt.Println("  server --web    # Start the web UI for uploading/managing content (web mode)")
}

func main() {
	fileMode := flag.Bool("file", false, "Serve feeds from a directory structure (filesystem mode)")
	webMode := flag.Bool("web", false, "Start the web UI for uploads and management")
	rootDir := flag.String("root", ".", "Root directory for content (used in --file mode)")
	addr := flag.String("addr", "0.0.0.0:8080", "Address to listen on")

	flag.Parse()

	if (*fileMode && *webMode) || (!*fileMode && !*webMode) {
		fmt.Println("You must specify exactly one mode.")
		printUsage()
		os.Exit(1)
	}

	if *fileMode {
		fmt.Printf("Starting in filesystem mode. Serving content from: %s\n", *rootDir)
		ServeFeedFromDir(*rootDir, *addr)
		return
	}

	if *webMode {
		fmt.Println("Web mode is not implemented yet. (Coming soon!)")
		os.Exit(1)
	}
}
