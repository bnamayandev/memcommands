#include <iostream>
#include <string>

// this is the node we used for the linked list
struct CommandNode {
   std::string command;
   CommandNode* next;
};

void addCommand(CommandNode*& head, const std::string& cmd) {}

void showHistory(CommandNode* head) {}

void freeHistory(CommandNode*& head) {}

int main() {
   CommandNode* history = nullptr;
   std::string input;

   std::cout << "Type 'history' or 'exit'\n";

   while (true) {
      std::cout << "> ";
      std::getline(std::cin, input);

      if (input == "exit") {
         break;
      }

      else if (input == "history") {
         showHistory(history);
         continue;
      }
   }

   freeHistory(history);
   return 0;
}
