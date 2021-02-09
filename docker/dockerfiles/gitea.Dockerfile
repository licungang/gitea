###! This dockerfile builds and starts a gitea service

FROM golang:1.15-alpine3.13 AS build-env

ARG GOPROXY="direct"
ARG TAGS="sqlite sqlite_unlock_notify bindata timetzdata"

ARG GITEA_VERSION
ARG CGO_EXTRA_CFLAGS

ARG GITEA_BUILDER_BUILD_DEPS="build-base nodejs npm git"

# Install build dependencies
RUN apk --no-cache add $GITEA_BUILDER_BUILD_DEPS

# Setup repo
COPY . "$GOPATH/src/code.gitea.io/gitea"

WORKDIR "$GOPATH/src/code.gitea.io/gitea"

# Checkout version if set
RUN [ -z "$GITEA_VERSION" ] || git checkout "${GITEA_VERSION}"

RUN make clean-all build

# DNM-CD(Krey): Implement automatic bumps of the alpine image
FROM alpine:3.13 AS gitea-service

LABEL maintainer="maintainers@gitea.io"

# File hierarchy
ARG GITEA_WORKDIR="/srv/gitea"
RUN mkdir -p "$GITEA_WORKDIR"

ARG GITEA_CUSTOMDIR="$GITEA_WORKDIR/custom"
RUN mkdir -p "$GITEA_CUSTOMDIR"

ARG GITEA_TEMPDIR="/var/tmp/gitea"
RUN mkdir -p "$GITEA_TEMPDIR"

ARG GITEA_CONFDIR="$GITEA_CUSTOMDIR/conf"
RUN mkdir -p "$GITEA_CONFDIR"

ARG GITEA_SRCDIR="/go/src/code.gitea.io/gitea"
ARG GITEA_APP_INI="$GITEA_CONFDIR/app.ini"
#COPY docker/files/confdir/gitea "$GITEA_CONFDIR"

RUN mkdir -p "$GITEA_WORKDIR"

ARG GITEA_EXECUTABLE="$GITEA_WORKDIR/gitea"

# Permission
ARG GITEA_USER="gitea"
ARG GITEA_USER_ID="1000"
ARG GITEA_USER_HOME="$GITEA_WORKDIR"
ARG GITEA_USER_SHELL="/bin/nologin"
ARG GITEA_GROUP="gitea"
ARG GITEA_GROUP_ID="1000"

# Dependencies
ARG GITEA_BUILD_DEPS="make go"
ARG GITEA_RUNTIME_DEPS="git"

# Install build dependencies
RUN apk --no-cache add $GITEA_BUILD_DEPS

# Install runtime dependencies
RUN apk --no-cache add $GITEA_RUNTIME_DEPS

# Create the gitea user
## NOTE(Krey): These are busybox commands so we have to first create group and then the user added to the group
RUN true \
	# addgroup [-g GID] [-S] [USER] GROUP
	&& addgroup \
		# Create a system group
		-S \
		# Group id
		-g "$GITEA_GROUP_ID" \
		"$GITEA_GROUP" \
	# adduser [OPTIONS] USER [GROUP]
	&& adduser \
		# Create System user
		-S \
		# Don't Create home directory
		-H \
		# Don't assign a password
		-D \
		# Home directory
		-h "$GITEA_USER_HOME" \
		# Login shell
		-s "$GITEA_USER_SHELL" \
		# User id
		-u "$GITEA_USER_ID" \
		# Group
		-G "$GITEA_GROUP" \
		"$GITEA_USER"

RUN chown -R "$GITEA_USER:$GITEA_GROUP" "$GITEA_USER_HOME"

# Copy the compiled source code in this container for installation
COPY --from=build-env "/go/src/code.gitea.io/gitea" "$GITEA_SRCDIR"

# Get gitea executable in the system
RUN cp "$GITEA_SRCDIR/gitea" "$GITEA_EXECUTABLE"

# Remove build dependencies
RUN apk del $GITEA_BUILD_DEPS

USER "$GITEA_USER"


# Configuration - GITEA_*
ARG GITEA_APP_NAME="Gitea: Git with a cup of tea"
ARG GITEA_RUN_USER="$GITEA_USER"
ARG GITEA_RUN_MODE="prod"

## [repository] - GITEA_REPO_*
ARG GITEA_REPO_ROOT="$GITEA_WORKDIR/git/repositories"

## [repository.local] - GITEA_LOCAL_REPO_*
ARG GITEA_LOCAL_REPO_PATH="/var/tmp/gitea/local-repo"

## [repository.upload] - GITEA_UPLOAD_REPO_*
ARG GITEA_UPLOAD_REPO_TEMP_PATH="/var/tmp/gitea/uploads"

## [server] - GITEA_SERVER_*
ARG GITEA_SERVER_APP_DATA_PATH="$GITEA_WORKDIR"
ARG GITEA_SERVER_SSH_DOMAIN="localhost"
ARG GITEA_SERVER_HTTP_PORT="3000"
ARG GITEA_SERVER_ROOT_URL="http://$GITEA_SSH_DOMAIN:$GITEA_HTTP_PORT/"
ARG GITEA_SERVER_DISABLE_SSH="false"
ARG GITEA_SERVER_START_SSH_SERVER="true"
ARG GITEA_SERVER_SSH_PORT="2222"
ARG GITEA_SERVER_SSH_LISTEN_PORT="2222"
# DNM(Krey): Create a user that handles git
ARG GITEA_SERVER_BUILTIN_SSH_SERVER_USER="git"
ARG GITEA_SERVER_LFS_START_SERVER="true"
ARG GITEA_SERVER_LFS_CONTENT_PATH="/var/lib/gitea/git/lfs"
ARG GITEA_SERVER_DOMAIN="localhost"
# DNM-SECURITY(Krey): This has to be auto-generated
ARG GITEA_SERVER_LFS_JWT_SECRET="kBHxlY89K3nkoTulGbBsDk7Ow_d6QKJxiKYnMWIhrD4"
ARG GITEA_SERVER_OFFLINE_MODE="false"

## [database]
ARG GITEA_DB_PATH="$GITEA_WORK_DIR/data/gitea.db"
ARG GITEA_DB_TYPE="mysql"
ARG GITEA_DB_HOST="db:3306"
ARG GITEA_DB_NAME="gitea"
# DNM-INVESTIGATE(Krey): Check if the database user should be the same as the user for the server -> Seems insane
ARG GITEA_DB_USER="gitea"
ARG GITEA_DB_PASSWD="gitea"
ARG GITEA_DB_SCHEMA
ARG GITEA_DB_SSL_MODE="disable"
ARG GITEA_DB_CHARSET="utf8mb4"
ARG GITEA_DB_LOG_SQL="false"

## [session] - GITEA_SESSION_*
ARG GITEA_SESSION_PROVIDER_CONFIG="/var/lib/gitea/data/sessions"
ARG GITEA_SESSION_PROVIDER="file"

## [picture] - GITEA_PICTURE_*
ARG GITEA_PICTURE_AVATAR_UPLOAD_PATH="/var/lib/gitea/data/avatars"
ARG GITEA_PICTURE_REPOSITORY_AVATAR_UPLOAD_PATH="/var/lib/gitea/data/gitea/repo-avatars"
ARG GITEA_PICTURE_DISABLE_GRAVATAR="false"
ARG GITEA_PICTURE_ENABLE_FEDERATED_AVATAR="true"

## [attachment] - GITEA_ATTACHMENT_*
ARG GITEA_ATTACHMENT_PATH="/var/lib/gitea/data/attachments"

## [log] - GITEA_LOG_*
ARG GITEA_LOG_ROOT_PATH="/var/lib/gitea/data/log"
ARG GITEA_LOG_MODE="console"
ARG GITEA_LOG_LEVEL="info"
ARG GITEA_LOG_ROUTER="console"

## [security] - GITEA_SECURITY_*
ARG GITEA_SECURITY_INSTALL_LOCK="true"
# DNM-SECURITY(Krey): This has to be auto-generated
ARG GITEA_SECURITY_SECRET_KEY="bFfPKzfkPfmGrr1pN6ZXrkqeetdXHGiZ0lnw9VrToAfFfwEKY9iXMJAzpWdJHE0C"
# DNM-SECURITY(Krey): This has to be auto-generated
ARG GITEA_SECURITY_INTERNAL_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYmYiOjE2MTI1MTMzNDN9.IN91i3MvGri3PabafVjX1ZbEB0TDvH8MioEuDSBAm04"

## [service] - GITEA_SERVICE_*
ARG GITEA_SERVICE_DISABLE_REGISTRATION="false"
ARG GITEA_SERVICE_REQUIRE_SIGNIN_VIEW="false"
ARG GITEA_SERVICE_REGISTER_EMAIL_CONFIRM="false"
ARG GITEA_SERVICE_ENABLE_NOTIFY_MAIL="false"
ARG GITEA_SERVICE_ALLOW_ONLY_EXTERNAL_REGISTRATION="false"
ARG GITEA_SERVICE_ENABLE_CAPTCHA="false"
ARG GITEA_SERVICE_DEFAULT_KEEP_EMAIL_PRIVATE="false"
ARG GITEA_SERVICE_DEFAULT_ALLOW_CREATE_ORGANIZATION="false"
ARG GITEA_SERVICE_DEFAULT_ENABLE_TIMETRACKING="false"
ARG GITEA_SERVICE_NO_REPLY_ADDRESS

## [oauth2] - GITEA_OAUTH2_*
# DNM-SECURITY(Krey): This has to be auto-generated
ARG GITEA_OAUTH2_JWT_SECRET="p7iYUHO-V3wNGTMGGtlXVa0OFn1avVTV6I6SAbSQh0o"

## [mailer] - GITEA_MAILER_*
ARG GITEA_MAILER_ENABLED="false"

## [openid] -- GITEA_OPENID_*
ARG GITEA_OPENID_ENABLE_OPENID_SIGNIN="false"
ARG GITEA_OPENID_ENABLE_OPENID_SIGNUP="false"

CMD [ "/srv/gitea/gitea", "web" ]