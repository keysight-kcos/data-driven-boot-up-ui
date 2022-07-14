package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"spirit-box/device"
	"spirit-box/logging"
	"spirit-box/scripts"
	"spirit-box/services"
	"spirit-box/tui"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/rs/cors"
)

const PORT = "8080"
const TEMP_PORT = "8081" // redirect port 80 output here while waiting for host UI to come up const CHANNEL_BUFFER = 100 const SYSTEMD_UPDATE_INTERVAL = 500 // in milliseconds

//go:embed frontend_build_files
var embeddedFiles embed.FS

func createSystemdHandler(uw *services.UnitWatcher) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(uw.Units)
	}
}

func createScriptsHandler(sc *scripts.ScriptController) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Print("Received req at scripts endpoint.")
		log.Printf("%v", *sc)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sc.PriorityGroups)
	}
}

func createQuitHandler(quit chan struct{}) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		quit <- struct{}{}
	}
}

func getFileSystem() http.FileSystem {
	fsys, err := fs.Sub(embeddedFiles, "frontend_build_files")
	if err != nil {
		panic(err)
	}

	return http.FS(fsys)
}

func setRules(addFlag, from, to string) error {
	// -A or -D for addFlag
	args := strings.Split(
		fmt.Sprintf("-t nat %s PREROUTING -i eth0 -p tcp --dport %s -j REDIRECT --to %s", addFlag, from, to), " ")
	cmd := exec.Command("iptables", args...)
	bytes, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("First: %w: %s, %v, %d", err, string(bytes), args, len(args))
	}

	args = strings.Split(
		fmt.Sprintf("-t nat %s OUTPUT -p tcp --dport %s -j REDIRECT --to %s", addFlag, from, to), " ")
	cmd = exec.Command("iptables", args...)
	bytes, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("Second: %w: %s, %v, %d", err, string(bytes), args, len(args))
	}

	return nil
}

func setPortForwarding() error {
	err := setRules("-A", "80", PORT)
	if err != nil {
		return err
	}
	return setRules("-A", TEMP_PORT, "80")
}

func unsetPortForwarding() error {
	err := setRules("-D", "80", PORT)
	if err != nil {
		return err
	}
	return setRules("-D", TEMP_PORT, "80")
}

func main() {
	quitWeb := make(chan struct{})
	quitTui := make(chan struct{})
	rebootServer := make(chan struct{})

	ip := device.GetIPv4Addr("eth0")
	ip = ip[:len(ip)-3]

	// apply iptables rules
	err := setPortForwarding()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			res, _ := http.Get(fmt.Sprintf("http://localhost:%s", TEMP_PORT))
			if res != nil { // Something's being served on port 80 (redirected to TEMP_PORT)
				err := unsetPortForwarding()
				if err != nil {
					log.Fatal(err)
				}
				rebootServer <- struct{}{}
				break
			}
			time.Sleep(time.Second)
		}
	}()

	dConn, err := dbus.New()
	if err != nil {
		log.Fatal(err)
	}
	defer dConn.Close()

	logging.InitLogger()
	uw := services.NewWatcher(dConn)
	sc := scripts.NewController()

	// setup endpoints for server
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(getFileSystem()))
	mux.HandleFunc("/systemd", createSystemdHandler(uw))
	mux.HandleFunc("/scripts", createScriptsHandler(sc))
	mux.HandleFunc("/quit", createQuitHandler(quitWeb))

	log.Printf("Starting server on port %s.", PORT)
	handler := cors.Default().Handler(mux)

	go func() { // start server, reboot if reboot message is sent
		for {
			s := http.Server{Addr: fmt.Sprintf(":%s", PORT), Handler: handler}
			go func() {
				time.Sleep(time.Duration(500) * time.Millisecond)
				<-rebootServer
				s.Shutdown(context.Background())
			}()
			if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatal("ListenAndServe:" + err.Error())
			}
		}
	}()

	// Writes default log messages (log.Print, log.Fatal, etc...)
	// to a file called tuiDebug.
	f, err := tea.LogToFile("tuiDebug", "debug")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	fmt.Printf("\033[2J") // clear the screen
	log.Print("Starting spirit-box...")
	uw.InitializeStates()
	go sc.RunPriorityGroups()

	var p *tea.Program
	go func(quit chan struct{}) {
		// the tui logic will "pump" the updates of the unit watcher.
		// no need to run uw.Start
		p = tui.CreateProgram(dConn, uw, ip, sc)
		if err := p.Start(); err != nil {
			fmt.Printf("There was an error: %v\n", err)
			os.Exit(1)
		}
		log.Print("Program exited.")
		quit <- struct{}{}
		log.Print("quit signal sent to channel.")
	}(quitTui)

	select {
	case <-quitWeb:
		p.Quit()
	case <-quitTui:
		break
	}

	log.Print("Cleanup.")
	unsetPortForwarding()

	// Dump log lines to stdout for dev purposes.
	fmt.Printf("\nLog Lines (in order of insertion):\n")
	for _, event := range logging.Logs.Events {
		fmt.Println(event.LogLine())
	}

	logFile := logging.CreateLogFile()
	defer logFile.Close()

	logging.Logs.WriteJSON(logFile)
	fmt.Printf("\nWrote JSON log entries to %s.\n", logFile.Name())
}
