#include <iostream>
#include <string>
#include <vector>
#include <cstdlib>
#include <fstream>

int main() {
   std::string input;
   std::string filePath;

   //get the shell the user is using
   std::string shellPath = std::getenv("SHELL");
   std::string shellName = shellPath.substr(shellPath.find_last_of('/') + 1);
   std::string home = std::getenv("HOME");
   std::vector<std::string> historyList;

   if (home.empty()) {
      std::cerr << "HOME environment variable is not set";
      return 1;
   }

   if (shellName == "bash") {
       filePath = home + "/.bash_history";
   } else if (shellName == "zsh") {
       filePath = home + "/.zsh_history";
   } else if (shellName == "fish") {
       filePath = home + "/.local/share/fish/fish_history";
   } else {
       std::cerr << "Unsupported shell\n";
       return 1;
   } 
   
   // read this history file
   std::ifstream file(filePath);

   if (!file.is_open()) {
      std::cerr << "Failed to open file: " << filePath << "\n";
      return 1;
   }

   std::string line;
   while (std::getline(file, line)) {
      historyList.push_back(line);
   }
   file.close();

   std::cout << historyList.size();

   return 0;
}
