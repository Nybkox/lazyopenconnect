package helpers

import (
	"context"
	"fmt"
	"time"

	"github.com/creativeprojects/go-selfupdate"

	"github.com/Nybkox/lazyopenconnect/pkg/version"
)

const repo = "Nybkox/lazyopenconnect"

type UpdateInfo struct {
	Available  bool
	Current    string
	Latest     string
	ReleaseURL string
}

func CheckForUpdate() (*UpdateInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create updater: %w", err)
	}

	latest, found, err := updater.DetectLatest(ctx, selfupdate.ParseSlug(repo))
	if err != nil {
		return nil, fmt.Errorf("failed to detect latest version: %w", err)
	}
	if !found {
		return &UpdateInfo{Available: false, Current: version.Current}, nil
	}

	if version.Current == "dev" {
		return &UpdateInfo{
			Available:  true,
			Current:    version.Current,
			Latest:     latest.Version(),
			ReleaseURL: latest.URL,
		}, nil
	}

	if latest.LessOrEqual(version.Current) {
		return &UpdateInfo{Available: false, Current: version.Current}, nil
	}

	return &UpdateInfo{
		Available:  true,
		Current:    version.Current,
		Latest:     latest.Version(),
		ReleaseURL: latest.URL,
	}, nil
}

func PerformUpdate() error {
	if IsHomebrewInstall() {
		return fmt.Errorf("installed via Homebrew - use: brew upgrade lazyopenconnect")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
	})
	if err != nil {
		return fmt.Errorf("failed to create updater: %w", err)
	}

	latest, found, err := updater.DetectLatest(ctx, selfupdate.ParseSlug(repo))
	if err != nil {
		return fmt.Errorf("failed to detect latest version: %w", err)
	}
	if !found {
		return fmt.Errorf("no release found")
	}

	if version.Current != "dev" && latest.LessOrEqual(version.Current) {
		return fmt.Errorf("already up to date (%s)", version.Current)
	}

	exe, err := selfupdate.ExecutablePath()
	if err != nil {
		return fmt.Errorf("could not locate executable: %w", err)
	}

	if err := updater.UpdateTo(ctx, latest, exe); err != nil {
		return fmt.Errorf("failed to update: %w", err)
	}

	return nil
}
