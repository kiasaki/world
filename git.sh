echo "\nFiles"
git log --format=format: --name-only --since="1 year ago" | sort | uniq -c | sort -nr | head -20
echo "\nPeople"
git shortlog -sn --no-merges
echo "\nBugs"
git log -i -E --grep="fix|bug|broken" --name-only --format='' | sort | uniq -c | sort -nr | head -20
echo "\nCommits"
git log --format='%ad' --date=format:'%Y-%m' | sort | uniq -c
echo "\nFire"
git log --oneline --since="1 year ago" | grep -iE 'revert|hotfix|emergency|rollback'
