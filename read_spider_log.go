package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/radovskyb/watcher"
)

var logDir string
var port int

func main() {
	flag.StringVar(&logDir, "logdir", "/var/log", "log directory to monitor")
	flag.IntVar(&port, "port", 8080, "port to listen on")
	flag.Parse()

	fmt.Printf("Monitoring log directory: %s\n", logDir)
	fmt.Printf("Listening on port: %d\n", port)

	go monitorLogDirectory()

	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data, err := generateDirectoryJSON(logDir)
		if err != nil {
			http.Error(w, "Failed to generate JSON data", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	http.ListenAndServe(fmt.Sprintf(":%d", port), r)
}

func monitorLogDirectory() {
	w := watcher.New()
	w.SetMaxEvents(1)
	w.FilterOps(watcher.Create, watcher.Write, watcher.Remove)

	go func() {
		for {
			select {
			case <-w.Event:
				fmt.Println("File change detected")
			case err := <-w.Error:
				fmt.Println("error:", err)
			case <-w.Closed:
				return
			}
		}
	}()

	if err := w.AddRecursive(logDir); err != nil {
		fmt.Println("Error adding directory to watcher:", err)
		return
	}

	w.IgnoreHiddenFiles(true)
	if err := w.Start(time.Millisecond * 100); err != nil {
		fmt.Println("Error starting watcher:", err)
		return
	}
}

func generateDirectoryJSON(root string) ([]byte, error) {
	data := make(map[string]interface{})
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		parts := strings.Split(relPath, string(filepath.Separator))
		curr := data

		if info.IsDir() {
			for i := 0; i < len(parts); i++ {
				if _, ok := curr[parts[i]]; !ok {
					curr[parts[i]] = make(map[string]interface{})
				}
				curr = curr[parts[i]].(map[string]interface{})
			}
		} else {
			content, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			for i := 0; i < len(parts)-1; i++ {
				if _, ok := curr[parts[i]]; !ok {
					curr[parts[i]] = make(map[string]interface{})
				}
				curr = curr[parts[i]].(map[string]interface{})
			}
			curr[parts[len(parts)-1]] = string(content)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}
