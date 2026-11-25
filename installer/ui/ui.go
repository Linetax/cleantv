// Package ui is how both installers talk to the person: a window (a small web page
// served on 127.0.0.1 and shown in an Edge or Chrome app window) or, with --console,
// the terminal. The installers call the same functions either way, so their logic does
// not depend on which one is showing.
package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// App describes the installer for the window: its name, the line under it and the
// promises shown on the welcome screen. Title is also the console banner.
type App struct {
	Title    string   `json:"title"`
	Name     string   `json:"name"`
	Tagline  string   `json:"tagline"`
	Features []string `json:"features"`
}

var (
	in  = bufio.NewReader(os.Stdin)
	win *server // nil in console mode
)

// Start shows the window, unless --console is on the command line (tests, terminals).
func Start(app App) {
	for _, a := range os.Args[1:] {
		if a == "--console" {
			fmt.Println("==============================================")
			fmt.Println("  " + app.Title)
			fmt.Println("==============================================")
			return
		}
	}
	win = newServer(app)
	win.start()
}

// Say prints one line.
func Say(s string) {
	if win != nil {
		win.emit(event{T: "line", Text: s})
		return
	}
	fmt.Println(s)
}

func Sayf(format string, a ...any) { Say(fmt.Sprintf(format, a...)) }

// Print writes without a newline (the dots of a wait). The window shows a spinner instead.
func Print(s string) {
	if win == nil {
		fmt.Print(s)
	}
}

// Step starts a new stage: a heading in the console, a new page in the window.
func Step(title string) {
	if win != nil {
		win.emit(event{T: "step", Text: title})
		return
	}
	fmt.Printf("\n--- %s ---\n", title)
}

// Ask reads a line, def when it is empty.
func Ask(q, def string) string {
	if win != nil {
		return win.ask(event{Kind: "text", Text: q, Def: def})
	}
	if def != "" {
		fmt.Printf("%s [%s] : ", q, def)
	} else {
		fmt.Printf("%s : ", q)
	}
	line, err := in.ReadString('\n')
	if err != nil && line == "" {
		fmt.Println()
		fmt.Println("ERREUR : Entrée fermée.")
		os.Exit(1)
	}
	if line = strings.TrimSpace(line); line == "" {
		return def
	}
	return line
}

// Pause waits for the person: Enter in the console, a button labelled button in the window.
func Pause(q, button string) {
	if win != nil {
		win.ask(event{Kind: "pause", Text: q, Button: button})
		return
	}
	Ask(q, "")
}

// Yes asks a yes/no question.
func Yes(q string, def bool) bool {
	if win != nil {
		d := "non"
		if def {
			d = "oui"
		}
		return win.ask(event{Kind: "yesno", Text: q, Def: d}) == "oui"
	}
	d := "o/N"
	if def {
		d = "O/n"
	}
	for {
		switch strings.ToLower(Ask(q+" ("+d+")", "")) {
		case "":
			return def
		case "o", "oui", "y", "yes":
			return true
		case "n", "non", "no":
			return false
		}
	}
}

// Choose lists numbered options and returns the number picked ("1", "2"...), def by default.
func Choose(q string, opts []string, def string) string {
	if win != nil {
		return win.ask(event{Kind: "choice", Text: q, Opts: opts, Def: def})
	}
	Say(q)
	for i, o := range opts {
		Sayf("  %d. %s", i+1, o)
	}
	return Ask("Votre choix", def)
}

// Finish ends the program once the person has read the last page.
func Finish() {
	if win != nil {
		win.end(event{T: "done"}, 0)
	}
	fmt.Println()
	Ask("Appuyez sur Entrée pour fermer", "")
	os.Exit(0)
}

// Die shows an error and ends the program.
func Die(msg string) {
	if win != nil {
		win.end(event{T: "error", Text: msg}, 1)
	}
	fmt.Println()
	fmt.Println("ERREUR : " + msg)
	fmt.Print("Appuyez sur Entrée pour fermer : ")
	in.ReadString('\n') // an error here only means there is nothing left to read

	os.Exit(1)
}
