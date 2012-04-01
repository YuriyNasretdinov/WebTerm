package main

/*
#cgo linux CFLAGS: -D__LINUX__
#cgo darwin CFLAGS: -D__DARWIN__
#cgo freebsd CFLAGS: -D__FREEBSD__
#cgo LDFLAGS: -lutil

#include <stdlib.h>
#include <sys/ioctl.h>

#ifdef __LINUX__
#include <pty.h>
#endif

#ifdef __DARWIN__
#include <util.h>
#endif

#ifdef __FREEBSD__
#include <sys/types.h>
#include <termios.h>
#include <libutil.h>
#endif

int goForkpty(int *amaster, struct winsize *winp) {
	return forkpty(amaster, NULL, NULL, winp);
}

int goChangeWinsz(int fd, struct winsize *winp) {
	return ioctl(fd, TIOCSWINSZ, winp);
}
*/
import "C"
import (
	"bytes"
	"fmt"
	"io"
	"crypto/md5"
	"os/exec"
	"net/http"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
	"code.google.com/p/go.net/websocket"
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

func setColsRows(winsz *C.struct_winsize, cols int, rows int) {
	winsz.ws_row = C.ushort(rows);
	winsz.ws_col = C.ushort(cols);
	winsz.ws_xpixel = C.ushort(cols * 9);
	winsz.ws_ypixel = C.ushort(rows * 16);
}

func redirToWs(fd int, ws *websocket.Conn) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Error occured: %s\n", r)
			runtime.Goexit()
		}
	}()
	
	var buf [8192]byte
	start, end, buflen := 0, 0, 0
	for {
		switch nr, er := syscall.Read(fd, buf[start:]); {
		case nr < 0:
			fmt.Fprintf(os.Stderr, "error reading from websocket %d with code %d\n", fd, er)
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

func redirFromWs(fd int, ws *websocket.Conn, pid int, winsz *C.struct_winsize) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Error occured: %s\n", r)
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
				syscall.Kill(pid, syscall.SIGHUP)
				return
			case nr > 0:
				nw, ew := syscall.Write(fd, buf[0:nr])
				if nw != nr {
					fmt.Fprintf(os.Stderr, "error writing to fd = %d with code %d\n", fd, ew)
					return
				}
			}
		case 'w':
			cols, rows := readInt(ws), readInt(ws)
			setColsRows(winsz, cols, rows)
			C.goChangeWinsz(C.int(fd), winsz)
		default:
			panic("Unknown command " + string(buf[0]))
		}
	}
}

var (
	bashrc = "bashrc"
	port = "12345"
	password_md5 = ""
	password_len = 0
	connections = 0
	willquit = false
)

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

	var winsz = new(C.struct_winsize)
	setColsRows(winsz, cols, rows)

	cpttyno := C.int(-1)
	pid := int(C.goForkpty(&cpttyno, winsz))
	pttyno := int(cpttyno)

	if pid == 0 {
		bashloc, err := exec.LookPath("bash")
		bashargs := []string{"bash", "--rcfile", bashrc}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not find bash: %s\n", err)
			bashloc = "/bin/sh"
			bashargs = []string{"sh"}
		}

		syscall.Exec(bashloc, bashargs, os.Environ())
		panic("unreachable code")
	}

	fmt.Println("Pid is", pid, " ptty number is", pttyno)
	go redirFromWs(pttyno, ws, pid, winsz)
	go redirToWs(pttyno, ws)
	syscall.Wait4(pid, nil, 0, nil)
	syscall.Close(pttyno)

	fmt.Println("Process finished")
}

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintf(os.Stderr, "Usage: %s <bashrc> <port> <password-md5> <password-len>\n", os.Args[0])
		os.Exit(1)
	}
	
	bashrc = os.Args[1]
	port = os.Args[2]
	password_md5 = os.Args[3]
	password_len, _ = strconv.Atoi(os.Args[4])
	
	fmt.Println("Started")
	go IdleQuitter()

	http.Handle("/ws", websocket.Handler(PtyServer))
	log.Fatal(http.ListenAndServe(":" + port, nil))
}
