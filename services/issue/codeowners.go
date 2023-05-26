package issue

import (
	"bufio"
	"context"
	b64 "encoding/base64"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/godo.v2/glob"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	organization_model "code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

type Codeowners struct {
	glob   string
	owners []string
}

func ParseCodeowners(changedFiles []string, codeownersContents []byte) ([]string, []string, error) {

	// This calls the actual parser
	globMap, err := ParseCodeownerBytes(codeownersContents)

	// We have to declare a new list of strings to be able to append all codeowners
	//	As we get them file by file in the following for loop
	var codeownersList []string
	for _, file := range changedFiles {
		codeownersList = append(codeownersList, GetOwners(globMap, file)...)
	}
	codeownerIndividuals, codeOwnerTeams := SeparateOwnerAndTeam(codeownersList)

	log.Trace("Final result of Codeowner Users: " + fmt.Sprint(codeownerIndividuals))
	log.Trace("Final result of Codeowner Teams: " + fmt.Sprint(codeOwnerTeams))

	return codeownerIndividuals, codeOwnerTeams, err

}

// GetOwners returns the list of owners (including teams) for a single file. It matches from our globMap
// to the changed files that it receives from the for loop in the ParseCodeowners function above.
func GetOwners(globMap []Codeowners, file string) []string {

	for i := len(globMap) - 1; i >= 0; i-- {
		if glob.Globexp(globMap[i].glob).MatchString(file) {
			fmt.Println("File:", file, "Result:", globMap[i])

			return globMap[i].owners
		}
	}
	log.Trace("!!!Unmatched file: ", file)
	return nil
}

// SeparateOwnerAndTeam separates owners and teams based on format.
// Note that it also calls RemoveDuplicateString to remove duplicates
func SeparateOwnerAndTeam(codeownersList []string) ([]string, []string) {

	codeownerIndividuals := []string{}
	codeOwnerTeams := []string{}

	codeownersList = RemoveDuplicateString(codeownersList)

	for _, codeowner := range codeownersList {

		if len(codeowner) > 0 {
			if strings.Compare(codeowner[0:1], "@") == 0 {
				codeowner = codeowner[1:]
			}

			if strings.Contains(codeowner, "/") {
				codeOwnerTeams = append(codeOwnerTeams, codeowner)
			} else {
				codeownerIndividuals = append(codeownerIndividuals, codeowner)
			}
		}
	}

	return codeownerIndividuals, codeOwnerTeams

}

// Removing duplicates has to be done manually in Golang
func RemoveDuplicateString(duplicatesPresent []string) []string {
	allKeys := make(map[string]bool)
	duplicatesRemoved := []string{}

	for _, item := range duplicatesPresent {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			duplicatesRemoved = append(duplicatesRemoved, item)
		}
	}

	return duplicatesRemoved

}

func ParseCodeownerBytes(codeownerBytes []byte) ([]Codeowners, error) {

	// Create a new scanner to read from the byte array
	scanner := bufio.NewScanner(strings.NewReader(string(codeownerBytes)))
	return ScanAndParse(*scanner)
}

func ScanAndParse(scanner bufio.Scanner) ([]Codeowners, error) {

	var globMap []Codeowners
	var lineCounter int = 0

	for scanner.Scan() {

		nextLine := scanner.Text()
		lineCounter++

		// strings.Fields() splits the string by whitespace
		splitStrings := strings.Fields(nextLine)
		var globString string
		var globString2 string
		var userStopIndex int
		var currFileUsers []string

		for i := 0; i < len(splitStrings); i++ {

			// fmt.Println(splitStrings[i])

			// The first two checks here handle comments
			if strings.Compare(splitStrings[i], "#") == 0 {
				break
			} else if strings.Contains(splitStrings[i], "#") {
				commentStrings := strings.Split(splitStrings[i], "#")
				if len(commentStrings[0]) > 0 {
					if i == 0 {
						globString = commentStrings[0]
					} else {
						splitStrings[i] = commentStrings[0]
						userStopIndex = i
					}
				}
				break

			} else if i == 0 {
				globString = splitStrings[i]

				// Note the logic here for mapping from Codeowners format to our current globbing library
				if len(globString) < 1 {
					// Can we handle a situation where the only file type is /?
					// I don't think so because I think that they would just have to use *
				} else if len(globString) == 1 {
					if strings.Compare(globString[0:1], "*") == 0 {
						globString = "**/**/**"
					}
				} else if strings.Compare(globString[0:1], "/") == 0 {
					globString = globString[1:] /*+ "**"*/
				} else if strings.Compare(globString[0:1], "*") == 0 &&
					strings.Compare(globString[1:2], "*") != 0 {
					globString = "**/" + globString
				} else if strings.Compare(globString[0:1], "*") != 0 {
					globString = "**/" + globString
				} else if strings.Compare(globString[(len(globString)-1):], "/") == 0 {
					globString = "**/" + globString + "**"
				}

				if strings.Compare(globString[len(globString)-1:], "/") != 0 &&
					strings.Compare(globString[len(globString)-1:], "*") != 0 {
					globString2 = globString + "/**"
				} else if strings.Compare(globString[len(globString)-1:], "/") == 0 {
					globString += "**"
				}

			} else {
				userStopIndex = i
			}

		}

		if userStopIndex > 0 {
			currFileUsers = splitStrings[1 : userStopIndex+1]
		}

		if len(currFileUsers) > 0 {
			validRule := true

			for _, user := range currFileUsers {
				if !glob.Globexp("@*").MatchString(user) {
					if !glob.Globexp("@*/*").MatchString(user) {
						if !glob.Globexp("*@*.*").MatchString(user) {
							validRule = false
						}
					}
				}
			}

			if validRule {
				newCodeowner := Codeowners{
					glob:   globString,
					owners: currFileUsers,
				}

				globMap = append(globMap, newCodeowner)

				if globString2 != "" {
					newCodeowner2 := Codeowners{
						glob:   globString2,
						owners: currFileUsers,
					}

					globMap = append(globMap, newCodeowner2)
				}
			} else {
				log.Trace("Invalid syntax given on line " + fmt.Sprint(lineCounter) + ":" +
					nextLine)
			}
		} else {

			newCodeowner := Codeowners{
				glob:   globString,
				owners: []string{""},
			}

			globMap = append(globMap, newCodeowner)
		}

		log.Trace("Line number " + fmt.Sprint(lineCounter) + ":")
		log.Trace("Parsed as Glob string: " + globString + "," + globString2 +
			"Users: " + fmt.Sprint(currFileUsers))
	}

	if scanner.Err() != nil {
		log.Trace(scanner.Err().Error())
		globMap = nil
		return globMap, scanner.Err()
	}

	log.Trace("Parsed map from codeowners file: " + fmt.Sprint(globMap))
	return globMap, scanner.Err()

}

// FindCodeownersFile gets the CODEOWNERS file from the top level,'.gitea', or 'docs' directory of the given repository.
func GetCodeownersFileContents(ctx context.Context, pr *issues_model.PullRequest, gitRepo *git.Repository) ([]byte, error) {
	possibleDirectories := []string{"", ".gitea/", "docs/"} // accepted directories to search for the CODEOWNERS file
	byteLimit := 3 * 1024 * 1024                            // 3 MB limit, per GitHub specs

	commit, err := gitRepo.GetCommit(pr.BaseBranch)
	if err != nil {
		return nil, err
	}

	entry := GetCodeownersGitTreeEntry(commit, possibleDirectories)
	if entry == nil {
		return nil, nil
	}

	if entry.IsRegular() {
		gitBlob := entry.Blob()
		data, err := gitBlob.GetBlobContentBase64()
		if err != nil {
			return nil, err
		}
		contentBytes, err := b64.StdEncoding.DecodeString(data)
		if err != nil {
			return nil, err
		}

		if len(contentBytes) >= byteLimit {
			log.Info("GetCodeownersFileContents [repo_id: %d, pr_id: %d, git_tree_entry_id: %d, content_num_bytes: %d, byte_limit: %d]: "+
				"CODEOWNERS file exceeds accepted size limit", pr.Issue.RepoID, pr.ID, entry.ID, len(contentBytes), byteLimit)
			return nil, nil
		}
		return contentBytes, nil
	}
	log.Warn("GetCodeownersFileContents [repo_id: %d, git_tree_entry_id: %d]: CODEOWNERS file found is not a regular file", pr.Issue.RepoID, entry.ID)

	return nil, nil
}

// GetCodeownersGitTreeEntry gets the git tree entry of the CODEOWNERS file, given an array of directories to search in. Nil if not found.
func GetCodeownersGitTreeEntry(commit *git.Commit, directoryOptions []string) *git.TreeEntry {
	for _, dir := range directoryOptions {
		entry, _ := commit.GetTreeEntryByPath(dir + "CODEOWNERS")
		if entry != nil {
			return entry
		}
	}
	return nil
}

// GetValidUserCodeownerReviewers gets the Users that actually exist, are authorized to review the pull request, and have explicit write permissions for the repo
func GetValidUserCodeownerReviewers(ctx context.Context, userNamesOrEmails []string, repo *repo_model.Repository, doer *user_model.User, isAdd bool, issue *issues_model.Issue) (reviewers []*user_model.User) {
	reviewers = []*user_model.User{}

	permDoer, err := access_model.GetUserRepoPermission(ctx, repo, doer)
	if err != nil {
		log.Error("models/perm/access/GetUserRepoPermission: %v", err)
		return reviewers // empty
	}

	for _, nameOrEmail := range userNamesOrEmails {
		var reviewer *user_model.User
		var err error
		if strings.Contains(nameOrEmail, "@") {
			reviewer, err = user_model.GetUserByEmail(ctx, nameOrEmail)
			if err != nil {
				log.Info("GetValidUserReviewers [repo_id: %d, owner_email: %s]: user owner in CODEOWNERS file could not be found by email", repo.ID, nameOrEmail)
			}
		} else {
			reviewer, err = user_model.GetUserByName(ctx, nameOrEmail)
			if err != nil {
				log.Info("GetValidUserReviewers [repo_id: %d, owner_username: %s]: user owner in CODEOWNERS file could not be found by name", repo.ID, nameOrEmail)
			}
		}
		if reviewer != nil && err == nil {
			err = IsValidUserCodeowner(err, ctx, reviewer, doer, isAdd, issue, permDoer, repo)
			if err == nil {
				reviewers = append(reviewers, reviewer)
			}
		}
	}
	return reviewers
}

// GetValidTeamCodeownerReviewers gets the Teams that actually exist, are authorized to review the pull request, and have explicit write permissions for the repo
func GetValidTeamCodeownerReviewers(ctx context.Context, fullTeamNames []string, repo *repo_model.Repository, doer *user_model.User, isAdd bool, issue *issues_model.Issue) (teamReviewers []*organization_model.Team) {
	teamReviewers = []*organization_model.Team{}
	if repo.Owner.IsOrganization() {
		for _, fullTeamName := range fullTeamNames {
			teamReviewer, err := GetTeamFromFullName(ctx, fullTeamName, doer)
			if err != nil {
				log.Info("GetTeamFromFullName [repo_id: %d, full_team_name: %s]: error finding the team [%v]", repo.ID, fullTeamName, err)
			} else if teamReviewer == nil {
				log.Info("GetTeamFromFullName [repo_id: %d, full_team_name: %s]: no error returned, but the team was nil (could not be found)", repo.ID, fullTeamName)
			} else {
				err = IsValidTeamCodeowner(ctx, teamReviewer, doer, isAdd, issue, repo)
				if err == nil {
					teamReviewers = append(teamReviewers, teamReviewer)
				}
			}
		}
	}
	return teamReviewers
}

// IsValidUserCodeowner returns an error if the user is not eligible to be a codeowner for the given repository (must be an eligible reviewer
// and have explciit write permissions). Nil if valid.
func IsValidUserCodeowner(err error, ctx context.Context, reviewer *user_model.User, doer *user_model.User, isAdd bool, issue *issues_model.Issue, permDoer access_model.Permission, repo *repo_model.Repository) error {
	err = IsValidReviewRequest(ctx, reviewer, doer, isAdd, issue, &permDoer)
	if err == nil {
		if UserHasWritePermissions(ctx, repo, reviewer) {
			return nil
		} else {
			log.Info("GetValidUserReviewers [repo_id: %d, user_id: %d]: user reviewer does not have write permissions and cannot be a codeowner", repo.ID, reviewer.ID)
		}
	} else {
		log.Info("GetValidUserReviewers [repo_id: %d, user_id: %d]: user reviewer is not a valid review request", repo.ID, reviewer.ID)
	}
	return errors.New(fmt.Sprintf("User %s is not a valid codeowner in the given repository", reviewer.Name))
}

// IsValidTeamCodeowner returns an error if the team is not eligible to be a codeowner for the given repository (must be an eligible reviewer
// and have explciit write permissions). Nil if valid.
func IsValidTeamCodeowner(ctx context.Context, teamReviewer *organization_model.Team, doer *user_model.User, isAdd bool, issue *issues_model.Issue, repo *repo_model.Repository) error {
	err := IsValidTeamReviewRequest(ctx, teamReviewer, doer, isAdd, issue)
	if err == nil {
		if TeamHasWritePermissions(ctx, repo, teamReviewer) {
			return nil
		} else {
			log.Info("GetValidTeamReviewers [repo_id: %d, team_id: %d]: team reviewer does not have write permissions and cannot be a codeowner", repo.ID, teamReviewer.ID)
		}
	} else {
		log.Info("GetValidTeamReviewers [repo_id: %d, team_id: %d]: team reviewer is not a valid review request", repo.ID, teamReviewer.ID)
	}
	return errors.New(fmt.Sprintf("Team %s is not a valid codeowner in the given repository", teamReviewer.Name))
}

// GetTeamFromFullName gets the team given its full name ('{organizationName}/{teamName}')
func GetTeamFromFullName(ctx context.Context, fullTeamName string, doer *user_model.User) (*organization_model.Team, error) {
	teamNameSplit := strings.Split(fullTeamName, "/")
	if len(teamNameSplit) != 2 {
		return nil, errors.New("Team name must split into exactly 2 parts when split on '/'")
	}
	organizationName, teamName := teamNameSplit[0], teamNameSplit[1]

	opts := organization_model.FindOrgOptions{
		ListOptions: db.ListOptions{
			ListAll: true,
		},
		UserID:         doer.ID,
		IncludePrivate: true,
	}
	organizations, err := organization_model.FindOrgs(opts)
	if err != nil {
		return nil, err
	}

	var organization *organization_model.Organization
	for _, org := range organizations {
		if org.Name == organizationName {
			organization = org
			break
		}
	}

	var team *organization_model.Team
	if organization != nil {
		team, err = organization.GetTeam(ctx, teamName)
		if err != nil {
			return nil, err
		}
	}
	return team, nil
}

// UserHasWritePermissions returns true if the user has write permissions to the code in the repository
func UserHasWritePermissions(ctx context.Context, repo *repo_model.Repository, user *user_model.User) bool {
	permission, err := access_model.GetUserRepoPermission(ctx, repo, user)
	if err != nil {
		log.Debug("models/perm/access/GetUserRepoPermission: %v", err)
		return false
	}
	return permission.CanWrite(unit.TypeCode)
}

// TeamHasWritePermissions returns true if the team has write permissions to the code in the repository
func TeamHasWritePermissions(ctx context.Context, repo *repo_model.Repository, team *organization_model.Team) bool {
	if organization_model.HasTeamRepo(ctx, team.OrgID, team.ID, repo.ID) {
		return team.UnitAccessMode(ctx, unit.TypeCode) == perm.AccessModeWrite
	} else {
		return false
	}
}
