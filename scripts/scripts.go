package scripts

import(
	"fmt"
	"log"
	"io/ioutil"
	"os/exec"
	"os"
	"errors"
	"bufio"
	"spirit-box/logging"
)

func RunAllScripts() {
	runScriptsInDir()
	loadScriptList()
}

func checkShebang(line string) (bool, string){
/*checks if the first 2 characters of a file are shebang
inputs: string - the file path
outputs: bool - true if shebang exists
         string - the path of the shell to use*/
	isScript := true
	file, err := os.Open(line)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Scan()
	shebang := scanner.Text()
	if shebang[:2] != "#!"{
		isScript = false
	}
	shell := shebang[2:]
	return isScript, shell
}

func executeAndOutput(l *log.Logger, line string) {
/*executes a script
inputs: *log.Logger - logger
        string - path to script*/
	isScript, shell := checkShebang(line)
	if !isScript{
		return
	}
	fmt.Println("Running script " + line + "...")
	out, err := exec.Command(shell, line).Output()
		if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s", out)
	l.Printf("Ran %s", line)
}

func runScriptsInDir(){
/*runs the scripts in hard coded directory*/
	l := logging.Logger
	scriptDir := "/usr/share/spirit-box/"
	items, _ := ioutil.ReadDir(scriptDir)
	fmt.Printf("Running scripts in %s\n", scriptDir);
	for _, item := range items {
		if !item.IsDir() && item.Name()[len(item.Name())-3:] == ".sh"{
			go executeAndOutput(l, scriptDir+item.Name())
		}
	}
	fmt.Println()
}

func loadScriptList() ([]string, error) {
/*executes scripts listed as paths in script file
outputs: []string - array of paths it attempts to execute
         error - errors*/
	var lines []string
	path := "/usr/share/spirit-box/scripts"
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist){
		fmt.Println("No script file.")
		return lines, err
	}

	fmt.Printf("Running scripts based on path names in %s\n", path)
	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	l := logging.Logger
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if _, err := os.Stat(line); errors.Is(err, os.ErrNotExist) {
			log.Fatal(errors.New("Script does not exist: " + line))
		} else {
			lines = append(lines, line)
			go executeAndOutput(l, line)
		}

	}
	fmt.Println()
	return lines, scanner.Err()
}
