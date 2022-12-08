// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cargo

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/log"
	packages_module "code.gitea.io/gitea/modules/packages"
	cargo_module "code.gitea.io/gitea/modules/packages/cargo"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/packages/helper"
	packages_service "code.gitea.io/gitea/services/packages"
	cargo_service "code.gitea.io/gitea/services/packages/cargo"
)

// https://doc.rust-lang.org/cargo/reference/registries.html#web-api
type StatusResponse struct {
	OK     bool            `json:"ok"`
	Errors []StatusMessage `json:"errors,omitempty"`
}

type StatusMessage struct {
	Message string `json:"detail"`
}

func apiError(ctx *context.Context, status int, obj interface{}) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.JSON(status, StatusResponse{
			OK: false,
			Errors: []StatusMessage{
				{
					Message: message,
				},
			},
		})
	})
}

type SearchResult struct {
	Crates []*SearchResultCrate `json:"crates"`
	Meta   SearchResultMeta     `json:"meta"`
}

type SearchResultCrate struct {
	Name          string `json:"name"`
	LatestVersion string `json:"max_version"`
	Description   string `json:"description"`
}

type SearchResultMeta struct {
	Total int64 `json:"total"`
}

// https://doc.rust-lang.org/cargo/reference/registries.html#search
func SearchPackages(ctx *context.Context) {
	page := ctx.FormInt("page")
	if page < 1 {
		page = 1
	}
	perPage := ctx.FormInt("per_page")
	paginator := db.ListOptions{
		Page:     page,
		PageSize: convert.ToCorrectPageSize(perPage),
	}

	pvs, total, err := packages_model.SearchLatestVersions(
		ctx,
		&packages_model.PackageSearchOptions{
			OwnerID:    ctx.Package.Owner.ID,
			Type:       packages_model.TypeCargo,
			Name:       packages_model.SearchValue{Value: ctx.FormTrim("q")},
			IsInternal: util.OptionalBoolFalse,
			Paginator:  &paginator,
		},
	)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	pds, err := packages_model.GetPackageDescriptors(ctx, pvs)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	crates := make([]*SearchResultCrate, 0, len(pvs))
	for _, pd := range pds {
		crates = append(crates, &SearchResultCrate{
			Name:          pd.Package.Name,
			LatestVersion: pd.Version.Version,
			Description:   pd.Metadata.(*cargo_module.Metadata).Description,
		})
	}

	ctx.JSON(http.StatusOK, SearchResult{
		Crates: crates,
		Meta: SearchResultMeta{
			Total: total,
		},
	})
}

type Owners struct {
	Users []OwnerUser `json:"users"`
}

type OwnerUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
}

// https://doc.rust-lang.org/cargo/reference/registries.html#owners-list
func ListOwners(ctx *context.Context) {
	ctx.JSON(http.StatusOK, Owners{
		Users: []OwnerUser{
			{
				ID:    ctx.Package.Owner.ID,
				Login: ctx.Package.Owner.Name,
				Name:  ctx.Package.Owner.DisplayName(),
			},
		},
	})
}

// DownloadPackageFile serves the content of a package
func DownloadPackageFile(ctx *context.Context) {
	s, pf, err := packages_service.GetFileStreamByPackageNameAndVersion(
		ctx,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeCargo,
			Name:        ctx.Params("package"),
			Version:     ctx.Params("version"),
		},
		&packages_service.PackageFileInfo{
			Filename: strings.ToLower(fmt.Sprintf("%s-%s.crate", ctx.Params("package"), ctx.Params("version"))),
		},
	)
	if err != nil {
		if err == packages_model.ErrPackageNotExist || err == packages_model.ErrPackageFileNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer s.Close()

	ctx.ServeContent(s, &context.ServeHeaderOptions{
		Filename:     pf.Name,
		LastModified: pf.CreatedUnix.AsLocalTime(),
	})
}

// https://doc.rust-lang.org/cargo/reference/registries.html#publish
func UploadPackage(ctx *context.Context) {
	defer ctx.Req.Body.Close()

	cp, err := cargo_module.ParsePackage(ctx.Req.Body)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	buf, err := packages_module.CreateHashedBufferFromReader(cp.Content, 32*1024*1024)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	pv, _, err := packages_service.CreatePackageAndAddFile(
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeCargo,
				Name:        cp.Name,
				Version:     cp.Version,
			},
			SemverCompatible: true,
			Creator:          ctx.Doer,
			Metadata:         cp.Metadata,
			VersionProperties: map[string]string{
				cargo_module.PropertyYanked: strconv.FormatBool(false),
			},
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: strings.ToLower(fmt.Sprintf("%s-%s.crate", cp.Name, cp.Version)),
			},
			Creator: ctx.Doer,
			Data:    buf,
			IsLead:  true,
		},
	)
	if err != nil {
		switch err {
		case packages_model.ErrDuplicatePackageVersion:
			apiError(ctx, http.StatusConflict, err)
		case packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
			apiError(ctx, http.StatusForbidden, err)
		default:
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	if err := cargo_service.AddOrUpdatePackageIndex(ctx, ctx.Doer, ctx.Package.Owner, pv.PackageID); err != nil {
		if err := packages_service.DeletePackageVersionAndReferences(ctx, pv); err != nil {
			log.Error("Rollback creation of package version: %v", err)
		}

		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, StatusResponse{OK: true})
}

// https://doc.rust-lang.org/cargo/reference/registries.html#yank
func YankPackage(ctx *context.Context) {
	yankPackage(ctx, true)
}

// https://doc.rust-lang.org/cargo/reference/registries.html#unyank
func UnyankPackage(ctx *context.Context) {
	yankPackage(ctx, false)
}

func yankPackage(ctx *context.Context, yank bool) {
	pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeCargo, ctx.Params("package"), ctx.Params("version"))
	if err != nil {
		if err == packages_model.ErrPackageNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	pps, err := packages_model.GetPropertiesByName(ctx, packages_model.PropertyTypeVersion, pv.ID, cargo_module.PropertyYanked)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pps) == 0 {
		apiError(ctx, http.StatusInternalServerError, "Property not found")
		return
	}

	pp := pps[0]
	pp.Value = strconv.FormatBool(yank)

	if err := packages_model.UpdateProperty(ctx, pp); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if err := cargo_service.AddOrUpdatePackageIndex(ctx, ctx.Doer, ctx.Package.Owner, pv.PackageID); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, StatusResponse{OK: true})
}
