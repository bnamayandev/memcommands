#include "./history/get_history.h"
#include "./fuzzy/input.h"
#include <iostream>

int main() {
    // 1) Load history lines from file (your function)
    std::vector<std::string> historyList = getHistoryLines();

    // 2) Start ncurses UI, user types, you recompute matches each keypress
    std::string finalQuery = runUI(historyList);

    // 3) After UI exits (user hit 'q' in your loop), you can do whatever
    std::cout << "Final query was: " << finalQuery << "\n";

    return 0;
}
