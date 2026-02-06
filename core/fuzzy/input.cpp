#include <ncurses.h>
#include <string>

std::string readQuery() { 
   initscr();
   cbreak();
   noecho();
   keypad(stdscr, true);

   std::string query;

   while (true) {
      move(0,0);
      printw("Type: %s", query.c_str());
      clrtoeol();
      refresh();

      int ch = getch();

      if (ch == 'q') break;
      
      if (ch == KEY_BACKSPACE || ch == 127 || ch == 8 ) {
         if (!query.empty()) query.pop_back();
         continue;
      }
      
      if (ch == '\n' || ch == '\r') continue;

      if (isprint(ch)) query.push_back(static_cast<char>(ch));
   }
   endwin();
   return query;
}
