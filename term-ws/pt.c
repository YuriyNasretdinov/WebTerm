// +build ignore

// Pseudo-terminal wrapper for term-ws
// Compilation:
//    gcc -lutil -D$(uname | tr '[a-z]' '[A-Z]') -o pt pt.c

#include <unistd.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <sys/socket.h>
#include <sys/select.h>
#include <stdio.h>

#include <stdlib.h>
#include <sys/ioctl.h>

#ifdef LINUX
#include <pty.h>
#endif

#ifdef DARWIN
#include <util.h>
#endif

#ifdef FREEBSD
#include <sys/types.h>
#include <termios.h>
#include <libutil.h>
#endif

static void set_fds(fd_set *reads, int pttyno, int have_stdin) {
    FD_ZERO(reads);
    if (have_stdin) FD_SET(0, reads);
    FD_SET(pttyno, reads);
}

int main(int argc, char *argv[]) {
    char buf[1024];
    int pttyno, n = 0;
    int pid, have_stdin = 1;
    struct winsize winsz;
    
    if (argc < 3) {
        fprintf(stderr, "Usage: %s <rows> <cols> <cmd> [args]\n", argv[0]);
        return 1;
    }
    
    winsz.ws_row = atoi(argv[1]);
    winsz.ws_col = atoi(argv[2]);
    winsz.ws_xpixel = winsz.ws_col * 9;
    winsz.ws_ypixel = winsz.ws_row * 16;
    
    pid = forkpty(&pttyno, NULL, NULL, &winsz);
    if (pid < 0) {
        perror("Cannot forkpty");
        return 1;
    } else if (pid == 0) {
        execvp(argv[3], argv + 3);
        perror("Cannot exec bash");
    }
    
    fd_set reads;
    set_fds(&reads, pttyno, have_stdin);
    
    while (select(pttyno + 1, &reads, NULL, NULL, NULL)) {
        if (FD_ISSET(0, &reads)) {
            n = read(0, buf, sizeof buf);
            if (n == 0) {
                have_stdin = 0;
            } else if (n < 0) {
                perror("Could not read from stdin");
                return 1;
            } else {
                if (write(pttyno, buf, n) != n) {
                    fprintf(stderr, "Not all bytes written into pseudoterminal\n");
                }
            }
        }
        
        if (FD_ISSET(pttyno, &reads)) {
            n = read(pttyno, buf, sizeof buf);
            if (n == 0) {
                break;
            } else if (n < 0) {
                break;
            }
            if (write(1, buf, n) != n) {
                fprintf(stderr, "Not all bytes read from pseudoterminal\n");
            }
        }
        
        set_fds(&reads, pttyno, have_stdin);
    }
    
    int statloc;
    wait(&statloc);
    
    return WEXITSTATUS(statloc);
}
