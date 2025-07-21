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
	fmt.Println()
	fmt.Println("Note: Both modes use the same directory structure and on-disk storage for content.")
	fmt.Println("      Web mode simply provides an admin panel for browser-based management.")
}

func main() {
	fileMode := flag.Bool("file", false, "Serve feeds from a directory structure (filesystem mode)")
	webMode := flag.Bool("web", false, "Start the web UI for uploads and management")
	rootDir := flag.String("root", ".", "Root directory for content (used in both modes)")
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
		fmt.Println("Starting in web mode. You'll be prompted to create your admin account.")
		StartWebServer(*addr, *rootDir)
		return
	}
}