// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/label"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ErrRepoLabelNotExist represents a "RepoLabelNotExist" kind of error.
type ErrRepoLabelNotExist struct {
	LabelID int64
	RepoID  int64
}

// IsErrRepoLabelNotExist checks if an error is a RepoErrLabelNotExist.
func IsErrRepoLabelNotExist(err error) bool {
	_, ok := err.(ErrRepoLabelNotExist)
	return ok
}

func (err ErrRepoLabelNotExist) Error() string {
	return fmt.Sprintf("label does not exist [label_id: %d, repo_id: %d]", err.LabelID, err.RepoID)
}

func (err ErrRepoLabelNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrOrgLabelNotExist represents a "OrgLabelNotExist" kind of error.
type ErrOrgLabelNotExist struct {
	LabelID int64
	OrgID   int64
}

// IsErrOrgLabelNotExist checks if an error is a OrgErrLabelNotExist.
func IsErrOrgLabelNotExist(err error) bool {
	_, ok := err.(ErrOrgLabelNotExist)
	return ok
}

func (err ErrOrgLabelNotExist) Error() string {
	return fmt.Sprintf("label does not exist [label_id: %d, org_id: %d]", err.LabelID, err.OrgID)
}

func (err ErrOrgLabelNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrLabelNotExist represents a "LabelNotExist" kind of error.
type ErrLabelNotExist struct {
	LabelID int64
}

// IsErrLabelNotExist checks if an error is a ErrLabelNotExist.
func IsErrLabelNotExist(err error) bool {
	_, ok := err.(ErrLabelNotExist)
	return ok
}

func (err ErrLabelNotExist) Error() string {
	return fmt.Sprintf("label does not exist [label_id: %d]", err.LabelID)
}

func (err ErrLabelNotExist) Unwrap() error {
	return util.ErrNotExist
}

// Label represents a label of repository for issues.
type Label struct {
	ID              int64 `xorm:"pk autoincr"`
	RepoID          int64 `xorm:"INDEX"`
	OrgID           int64 `xorm:"INDEX"`
	Name            string
	Exclusive       bool
	Description     string
	Color           string `xorm:"VARCHAR(7)"`
	NumIssues       int
	NumClosedIssues int
	CreatedUnix     timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix     timeutil.TimeStamp `xorm:"INDEX updated"`

	OriginalID int64 // Only for migrating data from other system, used for syncing

	NumOpenIssues     int    `xorm:"-"`
	NumOpenRepoIssues int64  `xorm:"-"`
	IsChecked         bool   `xorm:"-"`
	QueryString       string `xorm:"-"`
	IsSelected        bool   `xorm:"-"`
	IsExcluded        bool   `xorm:"-"`

	ArchivedUnix timeutil.TimeStamp `xorm:"DEFAULT NULL"`
}

func init() {
	db.RegisterModel(new(Label))
	db.RegisterModel(new(IssueLabel))
}

// CalOpenIssues sets the number of open issues of a label based on the already stored number of closed issues.
func (l *Label) CalOpenIssues() {
	l.NumOpenIssues = l.NumIssues - l.NumClosedIssues
}

// SetArchived set the label as archived
func (l *Label) SetArchived(isArchived bool) {
	if !isArchived {
		l.ArchivedUnix = timeutil.TimeStamp(0)
	} else if isArchived && l.ArchivedUnix.IsZero() {
		// Only change the date when it is newly archived.
		l.ArchivedUnix = timeutil.TimeStampNow()
	}
}

// CalOpenOrgIssues calculates the open issues of a label for a specific repo
func (l *Label) CalOpenOrgIssues(ctx context.Context, repoID, labelID int64) {
	counts, _ := CountIssuesByRepo(ctx, &IssuesOptions{
		RepoIDs:  []int64{repoID},
		LabelIDs: []int64{labelID},
		IsClosed: util.OptionalBoolFalse,
	})

	for _, count := range counts {
		l.NumOpenRepoIssues += count
	}
}

// LoadSelectedLabelsAfterClick calculates the set of selected labels when a label is clicked
func (l *Label) LoadSelectedLabelsAfterClick(currentSelectedLabels []int64, currentSelectedExclusiveScopes []string) {
	var labelQuerySlice []string
	labelSelected := false
	labelID := strconv.FormatInt(l.ID, 10)
	labelScope := l.ExclusiveScope()
	for i, s := range currentSelectedLabels {
		if s == l.ID {
			labelSelected = true
		} else if -s == l.ID {
			labelSelected = true
			l.IsExcluded = true
		} else if s != 0 {
			// Exclude other labels in the same scope from selection
			if s < 0 || labelScope == "" || labelScope != currentSelectedExclusiveScopes[i] {
				labelQuerySlice = append(labelQuerySlice, strconv.FormatInt(s, 10))
			}
		}
	}
	if !labelSelected {
		labelQuerySlice = append(labelQuerySlice, labelID)
	}
	l.IsSelected = labelSelected
	l.QueryString = strings.Join(labelQuerySlice, ",")
}

// BelongsToOrg returns true if label is an organization label
func (l *Label) BelongsToOrg() bool {
	return l.OrgID > 0
}

// IsArchived returns true if label is an archived
func (l *Label) IsArchived() bool {
	return l.ArchivedUnix > 0
}

// BelongsToRepo returns true if label is a repository label
func (l *Label) BelongsToRepo() bool {
	return l.RepoID > 0
}

// Return scope substring of label name, or empty string if none exists
func (l *Label) ExclusiveScope() string {
	if !l.Exclusive {
		return ""
	}
	lastIndex := strings.LastIndex(l.Name, "/")
	if lastIndex == -1 || lastIndex == 0 || lastIndex == len(l.Name)-1 {
		return ""
	}
	return l.Name[:lastIndex]
}

// NewLabel creates a new label
func NewLabel(ctx context.Context, l *Label) error {
	color, err := label.NormalizeColor(l.Color)
	if err != nil {
		return err
	}
	l.Color = color

	return db.Insert(ctx, l)
}

// NewLabels creates new labels
func NewLabels(ctx context.Context, labels ...*Label) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	for _, l := range labels {
		color, err := label.NormalizeColor(l.Color)
		if err != nil {
			return err
		}
		l.Color = color

		if err := db.Insert(ctx, l); err != nil {
			return err
		}
	}
	return committer.Commit()
}

// UpdateLabel updates label information.
func UpdateLabel(ctx context.Context, l *Label) error {
	color, err := label.NormalizeColor(l.Color)
	if err != nil {
		return err
	}
	l.Color = color

	return updateLabelCols(ctx, l, "name", "description", "color", "exclusive", "archived_unix")
}

// DeleteLabel delete a label
func DeleteLabel(ctx context.Context, id, labelID int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		l, err := GetLabelByID(ctx, labelID)
		if err != nil {
			if IsErrLabelNotExist(err) {
				return nil
			}
			return err
		}

		sess := db.GetEngine(ctx)

		if l.BelongsToOrg() && l.OrgID != id {
			return nil
		}
		if l.BelongsToRepo() && l.RepoID != id {
			return nil
		}

		if _, err = sess.ID(labelID).Delete(new(Label)); err != nil {
			return err
		}

		if _, err = sess.
			Where("label_id = ?", labelID).
			Delete(new(IssueLabel)); err != nil {
			return err
		}

		// delete comments about now deleted label_id
		if _, err = sess.Where("label_id = ?", labelID).Cols("label_id").Delete(&Comment{}); err != nil {
			return err
		}

		return nil
	})
}

// GetLabelByID returns a label by given ID.
func GetLabelByID(ctx context.Context, labelID int64) (*Label, error) {
	if labelID <= 0 {
		return nil, ErrLabelNotExist{labelID}
	}

	l := &Label{}
	has, err := db.GetEngine(ctx).ID(labelID).Get(l)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrLabelNotExist{l.ID}
	}
	return l, nil
}

// GetLabelsByIDs returns a list of labels by IDs
func GetLabelsByIDs(ctx context.Context, labelIDs []int64, cols ...string) ([]*Label, error) {
	labels := make([]*Label, 0, len(labelIDs))
	return labels, db.GetEngine(ctx).Table("label").
		In("id", labelIDs).
		Asc("name").
		Cols(cols...).
		Find(&labels)
}

// GetLabelInRepoByName returns a label by name in given repository.
func GetLabelInRepoByName(ctx context.Context, repoID int64, labelName string) (*Label, error) {
	if len(labelName) == 0 || repoID <= 0 {
		return nil, ErrRepoLabelNotExist{0, repoID}
	}

	l, exist, err := db.Get[Label](ctx, builder.Eq{"name": labelName, "repo_id": repoID})
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, ErrRepoLabelNotExist{0, repoID}
	}
	return l, nil
}

// GetLabelInRepoByID returns a label by ID in given repository.
func GetLabelInRepoByID(ctx context.Context, repoID, labelID int64) (*Label, error) {
	if labelID <= 0 || repoID <= 0 {
		return nil, ErrRepoLabelNotExist{labelID, repoID}
	}

	l, exist, err := db.Get[Label](ctx, builder.Eq{"id": labelID, "repo_id": repoID})
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, ErrRepoLabelNotExist{labelID, repoID}
	}
	return l, nil
}

// GetLabelIDsInRepoByNames returns a list of labelIDs by names in a given
// repository.
// it silently ignores label names that do not belong to the repository.
func GetLabelIDsInRepoByNames(ctx context.Context, repoID int64, labelNames []string) ([]int64, error) {
	labelIDs := make([]int64, 0, len(labelNames))
	return labelIDs, db.GetEngine(ctx).Table("label").
		Where("repo_id = ?", repoID).
		In("name", labelNames).
		Asc("name").
		Cols("id").
		Find(&labelIDs)
}

// BuildLabelNamesIssueIDsCondition returns a builder where get issue ids match label names
func BuildLabelNamesIssueIDsCondition(labelNames []string) *builder.Builder {
	return builder.Select("issue_label.issue_id").
		From("issue_label").
		InnerJoin("label", "label.id = issue_label.label_id").
		Where(
			builder.In("label.name", labelNames),
		).
		GroupBy("issue_label.issue_id")
}

// GetLabelsInRepoByIDs returns a list of labels by IDs in given repository,
// it silently ignores label IDs that do not belong to the repository.
func GetLabelsInRepoByIDs(ctx context.Context, repoID int64, labelIDs []int64) ([]*Label, error) {
	labels := make([]*Label, 0, len(labelIDs))
	return labels, db.GetEngine(ctx).
		Where("repo_id = ?", repoID).
		In("id", labelIDs).
		Asc("name").
		Find(&labels)
}

// GetLabelsByRepoID returns all labels that belong to given repository by ID.
func GetLabelsByRepoID(ctx context.Context, repoID int64, sortType string, listOptions db.ListOptions) ([]*Label, error) {
	if repoID <= 0 {
		return nil, ErrRepoLabelNotExist{0, repoID}
	}
	labels := make([]*Label, 0, 10)
	sess := db.GetEngine(ctx).Where("repo_id = ?", repoID)

	switch sortType {
	case "reversealphabetically":
		sess.Desc("name")
	case "leastissues":
		sess.Asc("num_issues")
	case "mostissues":
		sess.Desc("num_issues")
	default:
		sess.Asc("name")
	}

	if listOptions.Page != 0 {
		sess = db.SetSessionPagination(sess, &listOptions)
	}

	return labels, sess.Find(&labels)
}

// UpdateLabelsByRepoID adds, updates, and deletes relevant labels for the given repository.
func UpdateLabelsByRepoID(ctx context.Context, repoID int64, labels ...*Label) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		existingLabels, err := GetLabelsByRepoID(ctx, repoID, "", db.ListOptions{})
		if err != nil {
			return err
		}
		labelsToAdd := make([]*Label, 0)
		labelsToUpdate := make([]*Label, 0)
		labelsToDelete := make([]*Label, 0)

		for _, l := range labels {
			var foundLabel *Label
			for _, existingLabel := range existingLabels {
				if existingLabel.OriginalID == l.OriginalID {
					foundLabel = existingLabel
					break
				}
			}

			if foundLabel == nil {
				labelsToAdd = append(labelsToAdd, l)
			} else if foundLabel.Name != l.Name || foundLabel.Description != l.Description ||
				foundLabel.Color != l.Color {
				l.RepoID = repoID
				labelsToUpdate = append(labelsToUpdate, l)
			}
		}

		for _, existingLabel := range existingLabels {
			found := false
			for _, label := range labels {
				if label.OriginalID == existingLabel.OriginalID {
					found = true
					break
				}
			}
			if !found {
				labelsToDelete = append(labelsToDelete, existingLabel)
			}
		}

		for _, l := range labelsToAdd {
			if err = NewLabel(ctx, l); err != nil {
				return err
			}
		}

		for _, l := range labelsToUpdate {
			if err = UpdateLabel(ctx, l); err != nil {
				return err
			}
		}

		for _, l := range labelsToDelete {
			if err = DeleteLabel(ctx, repoID, l.ID); err != nil {
				return err
			}
		}
		return nil
	})
}

// CountLabelsByRepoID count number of all labels that belong to given repository by ID.
func CountLabelsByRepoID(ctx context.Context, repoID int64) (int64, error) {
	return db.GetEngine(ctx).Where("repo_id = ?", repoID).Count(&Label{})
}

// GetLabelInOrgByName returns a label by name in given organization.
func GetLabelInOrgByName(ctx context.Context, orgID int64, labelName string) (*Label, error) {
	if len(labelName) == 0 || orgID <= 0 {
		return nil, ErrOrgLabelNotExist{0, orgID}
	}

	l, exist, err := db.Get[Label](ctx, builder.Eq{"name": labelName, "org_id": orgID})
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, ErrOrgLabelNotExist{0, orgID}
	}
	return l, nil
}

// GetLabelInOrgByID returns a label by ID in given organization.
func GetLabelInOrgByID(ctx context.Context, orgID, labelID int64) (*Label, error) {
	if labelID <= 0 || orgID <= 0 {
		return nil, ErrOrgLabelNotExist{labelID, orgID}
	}

	l, exist, err := db.Get[Label](ctx, builder.Eq{"id": labelID, "org_id": orgID})
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, ErrOrgLabelNotExist{labelID, orgID}
	}
	return l, nil
}

// GetLabelIDsInOrgByNames returns a list of labelIDs by names in a given
// organization.
func GetLabelIDsInOrgByNames(ctx context.Context, orgID int64, labelNames []string) ([]int64, error) {
	if orgID <= 0 {
		return nil, ErrOrgLabelNotExist{0, orgID}
	}
	labelIDs := make([]int64, 0, len(labelNames))

	return labelIDs, db.GetEngine(ctx).Table("label").
		Where("org_id = ?", orgID).
		In("name", labelNames).
		Asc("name").
		Cols("id").
		Find(&labelIDs)
}

// GetLabelsInOrgByIDs returns a list of labels by IDs in given organization,
// it silently ignores label IDs that do not belong to the organization.
func GetLabelsInOrgByIDs(ctx context.Context, orgID int64, labelIDs []int64) ([]*Label, error) {
	labels := make([]*Label, 0, len(labelIDs))
	return labels, db.GetEngine(ctx).
		Where("org_id = ?", orgID).
		In("id", labelIDs).
		Asc("name").
		Find(&labels)
}

// GetLabelsByOrgID returns all labels that belong to given organization by ID.
func GetLabelsByOrgID(ctx context.Context, orgID int64, sortType string, listOptions db.ListOptions) ([]*Label, error) {
	if orgID <= 0 {
		return nil, ErrOrgLabelNotExist{0, orgID}
	}
	labels := make([]*Label, 0, 10)
	sess := db.GetEngine(ctx).Where("org_id = ?", orgID)

	switch sortType {
	case "reversealphabetically":
		sess.Desc("name")
	case "leastissues":
		sess.Asc("num_issues")
	case "mostissues":
		sess.Desc("num_issues")
	default:
		sess.Asc("name")
	}

	if listOptions.Page != 0 {
		sess = db.SetSessionPagination(sess, &listOptions)
	}

	return labels, sess.Find(&labels)
}

// GetLabelIDsByNames returns a list of labelIDs by names.
// It doesn't filter them by repo or org, so it could return labels belonging to different repos/orgs.
// It's used for filtering issues via indexer, otherwise it would be useless.
// Since it could return labels with the same name, so the length of returned ids could be more than the length of names.
func GetLabelIDsByNames(ctx context.Context, labelNames []string) ([]int64, error) {
	labelIDs := make([]int64, 0, len(labelNames))
	return labelIDs, db.GetEngine(ctx).Table("label").
		In("name", labelNames).
		Cols("id").
		Find(&labelIDs)
}

// CountLabelsByOrgID count all labels that belong to given organization by ID.
func CountLabelsByOrgID(ctx context.Context, orgID int64) (int64, error) {
	return db.GetEngine(ctx).Where("org_id = ?", orgID).Count(&Label{})
}

func updateLabelCols(ctx context.Context, l *Label, cols ...string) error {
	_, err := db.GetEngine(ctx).ID(l.ID).
		SetExpr("num_issues",
			builder.Select("count(*)").From("issue_label").
				Where(builder.Eq{"label_id": l.ID}),
		).
		SetExpr("num_closed_issues",
			builder.Select("count(*)").From("issue_label").
				InnerJoin("issue", "issue_label.issue_id = issue.id").
				Where(builder.Eq{
					"issue_label.label_id": l.ID,
					"issue.is_closed":      true,
				}),
		).
		Cols(cols...).Update(l)
	return err
}
