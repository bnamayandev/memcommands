#include <string>
#include <cstdlib>

// Computes how close a query is to the target, where the target
// is what the user is currently inputting, and the queries are
// all the previous commands the user has in their history.
// a positive score is returned if all the characters are returned in oreder,
// a -1 is returned if there is no characters in order or subsequence at all
int fuzzyScore(const std::string& query, const std::string& target) {
    int score = 0;
    int qi = 0;
    int lastMatch = -1;
    int consecutive = 0;

    for (int ti = 0; ti < target.size() && qi < query.size(); ti++) {
        if (tolower(target[ti]) == tolower(query[qi])) {
            score += 5;

            if (lastMatch == ti - 1) {
                consecutive++;
                score += consecutive * 15;
            } else {
                consecutive = 0;
                score -= (ti - lastMatch);
            }

            if (ti == 0 || target[ti - 1] == ' ' ||
                target[ti - 1] == '/' || target[ti - 1] == '-') {
                score += 10;
            }

            lastMatch = ti;
            qi++;
        }
    }

    return (qi == query.size()) ? score : -1;
}
