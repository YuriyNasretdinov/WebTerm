package main

import (
	"bytes"
	"code.google.com/p/go.net/websocket"
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

var (
	bashrc       = "bashrc"
	port         = "12345"
	password_md5 = ""
	password_len = 0
	connections  = 0
	willquit     = false
	ptPath       = "./pt"
)

func readFull(r io.Reader, buf []byte) {
	_, err := io.ReadFull(r, buf)
	if err != nil {
		panic(fmt.Sprintf("Could not read fully from %v: %s", r, err))
	}
}

func readInt(r io.Reader) int {
	var buf [8]byte
	readFull(r, buf[:])
	result, err := strconv.Atoi(strings.TrimSpace(string(buf[:])))
	if err != nil {
		panic(fmt.Sprintf("Could not convert input from %v to int: %s", r, err))
	}
	return result
}

func redirToWs(stdoutPipe io.Reader, ws *websocket.Conn) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Error occured: %s\n", r)
			runtime.Goexit()
		}
	}()

	var buf [8192]byte
	start, end, buflen := 0, 0, 0
	for {
		switch nr, err := stdoutPipe.Read(buf[start:]); {
		case nr < 0:
			fmt.Fprintf(os.Stderr, "error reading from stdoutPipe: %s\n", err.Error())
			return
		case nr == 0: // EOF
			return
		case nr > 0:
			buflen = start + nr
			for end = buflen - 1; end >= 0; end-- {
				if utf8.RuneStart(buf[end]) {
					ch, width := utf8.DecodeRune(buf[end:buflen])
					if ch != utf8.RuneError {
						end += width
					}
					break
				}

				if buflen-end >= 6 {
					fmt.Fprintf(os.Stderr, "Invalid UTF-8 sequence in output")
					end = nr
					break
				}
			}

			runes := bytes.Runes(buf[0:end])
			buf_clean := []byte(string(runes))

			nw, ew := ws.Write(buf_clean[:])
			if ew != nil {
				fmt.Fprintf(os.Stderr, "error writing to websocket with code %s\n", ew)
				return
			}

			if nw != len(buf_clean) {
				fmt.Fprintf(os.Stderr, "Written %d instead of expected %d\n", nw, end)
			}

			start = buflen - end

			if start > 0 {
				// copy remaning read bytes from the end to the beginning of a buffer
				// so that we will get normal bytes
				for i := 0; i < start; i++ {
					buf[i] = buf[end+i]
				}
			}
		}
	}
}

func redirFromWs(stdinPipe io.Writer, ws *websocket.Conn, pid int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Error occured: %s\n", r)
			syscall.Kill(pid, syscall.SIGHUP)
			runtime.Goexit()
		}
	}()

	var buf [2048]byte
	for {
		/*
			communication protocol:

			1 byte   cmd

			if cmd = i // input
				8 byte        length (ascii)
				length bytes  the actual input

			if cmd = w // window size changed
				8 byte        cols (ascii)
				8 byte        rows (ascii)
		*/

		readFull(ws, buf[0:1])

		switch buf[0] {
		case 'i':
			length := readInt(ws)
			switch nr, er := io.ReadFull(ws, buf[0:length]); {
			case nr < 0:
				fmt.Fprintf(os.Stderr, "error reading from websocket with code %s\n", er)
				return
			case nr == 0: // EOF
				fmt.Fprintf(os.Stderr, "connection closed, sending SIGHUP to %d\n")
				syscall.Kill(pid, syscall.SIGHUP)
				return
			case nr > 0:
				_, err := stdinPipe.Write(buf[0:nr])
				if err != nil {
					fmt.Fprintf(os.Stderr, "error writing to stdinPipe: %s\n", err.Error())
					return
				}
			}
		case 'w':
			// sadly, we can no longer change window size as easily :(
			// cols, rows := readInt(ws), readInt(ws)
			// setColsRows(winsz, cols, rows)
			// C.goChangeWinsz(C.int(fd), winsz)
		default:
			panic("Unknown command " + string(buf[0]))
		}
	}
}

func IdleQuitter() {

	for {
		if connections == 0 {
			if willquit {
				fmt.Println("No connections for long time, exiting")
				os.Exit(0)
			} else {
				willquit = true
			}
		} else {
			willquit = false
		}

		time.Sleep(time.Minute)
	}
}

func PtyServer(ws *websocket.Conn) {
	connections++
	willquit = false
	defer func() {
		connections--

		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Error occured: %s\n", r)
			runtime.Goexit()
		}
	}()

	fmt.Println("New client")

	// password needs to be supplied before connection
	passbuf := make([]byte, password_len)
	readFull(ws, passbuf)
	h := md5.New()
	h.Write(passbuf)
	if fmt.Sprintf("%x", h.Sum(nil)) != password_md5 {
		panic("Password incorrect")
	}
	// reading rows and cols
	cols, rows := readInt(ws), readInt(ws)

	args := []string{fmt.Sprint(rows), fmt.Sprint(cols)}

	_, err := exec.LookPath("bash")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not find bash: %s\n", err)
		args = append(args, "sh")
	} else {
		args = append(args, "bash", "--rcfile", bashrc)
	}

	cmd := exec.Command(ptPath, args...)
	inPipe, err := cmd.StdinPipe()
	if err != nil {
		panic("Cannot create stdin pipe")
	}
	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		panic("Cannot create stdout pipe")
	}
	cmd.Stderr = os.Stdout

	err = cmd.Start()
	if err != nil {
		panic("Could not start process: " + err.Error())
	}

	pid := cmd.Process.Pid
	fmt.Println("Pid is", pid)

	go redirFromWs(inPipe, ws, pid)
	go redirToWs(outPipe, ws)

	err = cmd.Wait()
	if err != nil {
		fmt.Println("Process finished with error: " + err.Error())
	}

	fmt.Println("Process finished")
}

func main() {
	if len(os.Args) != 6 {
		fmt.Fprintf(os.Stderr, "Usage: %s <bashrc> <port> <password-md5> <password-len> <pt-path>\n", os.Args[0])
		os.Exit(1)
	}

	bashrc = os.Args[1]
	port = os.Args[2]
	password_md5 = os.Args[3]
	password_len, _ = strconv.Atoi(os.Args[4])
	ptPath = os.Args[5]

	fmt.Println("Started")
	go IdleQuitter()

	http.Handle("/ws", websocket.Handler(PtyServer))
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
