#ifndef FZF_H
#define FZF_H

#include <string>
#include <vector>

int fuzzyScore(const std::string& query, const std::string& target);

std::vector<std::pair<int, std::string>> getFuzzyScoreList(std::vector<std::string> commandHistory, std::string query);

#endif
