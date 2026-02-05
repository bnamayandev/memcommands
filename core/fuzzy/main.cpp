#include <string>
#include <iostream>
#include <cstdlib>
#include "fzf.h"

int main() {
   std::cout << fuzzyScore("git", "git commit -m");
   return 0;
}
