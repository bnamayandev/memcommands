#include <cctype>
#include <string>
#include <vector>
#include <algorithm>
#include <utility>

int fuzzyScore(const std::string& query, const std::string& target) {
    auto lc = [](unsigned char c) { return std::tolower(c); };

    int score = 0;
    int qi = 0;
    int lastMatch = -1;
    int consecutive = 0;

    for (int ti = 0; ti < (int)target.size() && qi < (int)query.size(); ti++) {
        score -= 1; 

        if (lc((unsigned char)target[ti]) == lc((unsigned char)query[qi])) {
            score += 5;

            if (lastMatch == ti - 1) {
                consecutive++;
                score += consecutive * 15;
            } else {
                consecutive = 0;
                int gap = (lastMatch == -1) ? ti : (ti - lastMatch - 1);
                score -= gap; 
            }

            if (ti == 0 || target[ti - 1] == ' ' ||
                target[ti - 1] == '/' || target[ti - 1] == '-') {
                score += 10;
            }

            lastMatch = ti;
            qi++;
        }
    }

    return (qi == (int)query.size()) ? score : -1;
}

std::vector<std::pair<int, std::string>> getFuzzyScoreList(std::vector<std::string> commandHistory, std::string query) {
  int numberOfCommands = commandHistory.size();

  std::vector<std::pair<int, std::string>> indexedList;

  for (int i = 0; i < numberOfCommands; i++){
     std::string word = commandHistory[i];
     int score = fuzzyScore(query, word);
     indexedList.push_back({score, word});
  }


  std::sort(indexedList.begin(), indexedList.end(), [](const auto& a, const auto& b) { return a.first > b.first; });
  return indexedList; 
}
