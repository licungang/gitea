// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rpm

import (
	stdctx "context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	packages_module "code.gitea.io/gitea/modules/packages"
	rpm_module "code.gitea.io/gitea/modules/packages/rpm"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/packages/helper"
	notify_service "code.gitea.io/gitea/services/notify"
	packages_service "code.gitea.io/gitea/services/packages"
	rpm_service "code.gitea.io/gitea/services/packages/rpm"
)

func apiError(ctx *context.Context, obj any, statuses ...int) {
	status := helper.FormResponseCode(obj, statuses...)
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.PlainText(status, message)
	})
}

// https://dnf.readthedocs.io/en/latest/conf_ref.html
func GetRepositoryConfig(ctx *context.Context) {
	url := fmt.Sprintf("%sapi/packages/%s/rpm", setting.AppURL, ctx.Package.Owner.Name)

	ctx.PlainText(http.StatusOK, `[gitea-`+ctx.Package.Owner.LowerName+`]
name=`+ctx.Package.Owner.Name+` - `+setting.AppName+`
baseurl=`+url+`
enabled=1
gpgcheck=1
gpgkey=`+url+`/repository.key`)
}

// Gets or creates the PGP public key used to sign repository metadata files
func GetRepositoryKey(ctx *context.Context) {
	_, pub, err := rpm_service.GetOrCreateKeyPair(ctx, ctx.Package.Owner.ID)
	if err != nil {
		apiError(ctx, err)
		return
	}

	ctx.ServeContent(strings.NewReader(pub), &context.ServeHeaderOptions{
		ContentType: "application/pgp-keys",
		Filename:    "repository.key",
	})
}

// Gets a pre-generated repository metadata file
func GetRepositoryFile(ctx *context.Context) {
	pv, err := rpm_service.GetOrCreateRepositoryVersion(ctx, ctx.Package.Owner.ID)
	if err != nil {
		apiError(ctx, err)
		return
	}

	s, u, pf, err := packages_service.GetFileStreamByPackageVersion(
		ctx,
		pv,
		&packages_service.PackageFileInfo{
			Filename: ctx.Params("filename"),
		},
	)
	if err != nil {
		apiError(ctx, err)
		return
	}

	helper.ServePackageFile(ctx, s, u, pf)
}

func UploadPackageFile(ctx *context.Context) {
	upload, close, err := ctx.UploadStream()
	if err != nil {
		apiError(ctx, err)
		return
	}
	if close {
		defer upload.Close()
	}

	buf, err := packages_module.CreateHashedBufferFromReader(upload)
	if err != nil {
		apiError(ctx, err)
		return
	}
	defer buf.Close()

	pck, err := rpm_module.ParsePackage(buf)
	if err != nil {
		apiError(ctx, err)
		return
	}

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		apiError(ctx, err)
		return
	}

	fileMetadataRaw, err := json.Marshal(pck.FileMetadata)
	if err != nil {
		apiError(ctx, err)
		return
	}

	_, _, err = packages_service.CreatePackageOrAddFileToExisting(
		ctx,
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeRpm,
				Name:        pck.Name,
				Version:     pck.Version,
			},
			Creator:  ctx.Doer,
			Metadata: pck.VersionMetadata,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: fmt.Sprintf("%s-%s.%s.rpm", pck.Name, pck.Version, pck.FileMetadata.Architecture),
			},
			Creator: ctx.Doer,
			Data:    buf,
			IsLead:  true,
			Properties: map[string]string{
				rpm_module.PropertyMetadata: string(fileMetadataRaw),
			},
		},
	)
	if err != nil {
		apiError(ctx, err)
		return
	}

	if err := rpm_service.BuildRepositoryFiles(ctx, ctx.Package.Owner.ID); err != nil {
		apiError(ctx, err)
		return
	}

	ctx.Status(http.StatusCreated)
}

func DownloadPackageFile(ctx *context.Context) {
	name := ctx.Params("name")
	version := ctx.Params("version")

	s, u, pf, err := packages_service.GetFileStreamByPackageNameAndVersion(
		ctx,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeRpm,
			Name:        name,
			Version:     version,
		},
		&packages_service.PackageFileInfo{
			Filename: fmt.Sprintf("%s-%s.%s.rpm", name, version, ctx.Params("architecture")),
		},
	)
	if err != nil {
		apiError(ctx, err)
		return
	}

	helper.ServePackageFile(ctx, s, u, pf)
}

func DeletePackageFile(webctx *context.Context) {
	name := webctx.Params("name")
	version := webctx.Params("version")
	architecture := webctx.Params("architecture")

	var pd *packages_model.PackageDescriptor

	err := db.WithTx(webctx, func(ctx stdctx.Context) error {
		pv, err := packages_model.GetVersionByNameAndVersion(ctx, webctx.Package.Owner.ID, packages_model.TypeRpm, name, version)
		if err != nil {
			return err
		}

		pf, err := packages_model.GetFileForVersionByName(
			ctx,
			pv.ID,
			fmt.Sprintf("%s-%s.%s.rpm", name, version, architecture),
			packages_model.EmptyFileKey,
		)
		if err != nil {
			return err
		}

		if err := packages_service.DeletePackageFile(ctx, pf); err != nil {
			return err
		}

		has, err := packages_model.HasVersionFileReferences(ctx, pv.ID)
		if err != nil {
			return err
		}
		if !has {
			pd, err = packages_model.GetPackageDescriptor(ctx, pv)
			if err != nil {
				return err
			}

			if err := packages_service.DeletePackageVersionAndReferences(ctx, pv); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		apiError(webctx, err)
		return
	}

	if pd != nil {
		notify_service.PackageDelete(webctx, webctx.Doer, pd)
	}

	if err := rpm_service.BuildRepositoryFiles(webctx, webctx.Package.Owner.ID); err != nil {
		apiError(webctx, err)
		return
	}

	webctx.Status(http.StatusNoContent)
}
