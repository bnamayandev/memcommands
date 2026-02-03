#include <ncurses.h>
#include <string>

// Full-screen UI mode
void openUI() {
    clear();
    box(stdscr, 0, 0);
    mvprintw(1, 2, "Command History UI");
    mvprintw(3, 2, "Press 'q' to exit");
    refresh();

    int ch;
    while ((ch = getch()) != 'q') {
        // Locked here until 'q'
    }

    clear();
    refresh();
}

int main() {
    // Start ncurses
    initscr();
    raw();          // Disable line buffering
    noecho();       // Don't echo keys
    keypad(stdscr, TRUE);

    printw("Press Ctrl+P to open UI, Ctrl+C to quit\n");
    refresh();

    int ch;
    while (true) {
        ch = getch();

        // Ctrl+P = ASCII 16
        if (ch == 16) {
            openUI();
            printw("Returned to CLI mode\n");
            refresh();
        }
    }

    endwin();
    return 0;
}
