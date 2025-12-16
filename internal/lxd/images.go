package lxd

import (
	"context"
	"fmt"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
)

// ListImages returns a list of all images on the server
func ListImages(ctx context.Context, server lxd.InstanceServer) ([]api.Image, error) {
	images, err := server.GetImages()
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}
	return images, nil
}

// GetImage retrieves a specific image by fingerprint
func GetImage(ctx context.Context, server lxd.InstanceServer, fingerprint string) (*api.Image, string, error) {
	image, etag, err := server.GetImage(fingerprint)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get image %s: %w", fingerprint, err)
	}
	return image, etag, nil
}

// GetImageByAlias retrieves an image by its alias
func GetImageByAlias(ctx context.Context, server lxd.InstanceServer, alias string) (*api.Image, string, error) {
	// First get the alias to find the fingerprint
	aliasInfo, _, err := server.GetImageAlias(alias)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get image alias %s: %w", alias, err)
	}

	return GetImage(ctx, server, aliasInfo.Target)
}

// CopyImageFromRemote copies an image from a remote server to the local server
func CopyImageFromRemote(ctx context.Context, server lxd.InstanceServer, remoteServer lxd.ImageServer, fingerprint string, aliases []string) error {
	image, _, err := remoteServer.GetImage(fingerprint)
	if err != nil {
		return fmt.Errorf("failed to get remote image %s: %w", fingerprint, err)
	}

	// Build aliases
	imageAliases := make([]api.ImageAlias, len(aliases))
	for i, alias := range aliases {
		imageAliases[i] = api.ImageAlias{Name: alias}
	}

	args := lxd.ImageCopyArgs{
		Aliases: imageAliases,
		Public:  false,
	}

	op, err := server.CopyImage(remoteServer, *image, &args)
	if err != nil {
		return fmt.Errorf("failed to copy image %s: %w", fingerprint, err)
	}

	if err := op.Wait(); err != nil {
		return fmt.Errorf("failed waiting for image copy: %w", err)
	}

	return nil
}

// DeleteImage deletes an image by fingerprint
func DeleteImage(ctx context.Context, server lxd.InstanceServer, fingerprint string) error {
	op, err := server.DeleteImage(fingerprint)
	if err != nil {
		return fmt.Errorf("failed to delete image %s: %w", fingerprint, err)
	}

	if err := op.Wait(); err != nil {
		return fmt.Errorf("failed waiting for image deletion: %w", err)
	}

	return nil
}

// ListImageAliases returns all image aliases on the server
func ListImageAliases(ctx context.Context, server lxd.InstanceServer) ([]api.ImageAliasesEntry, error) {
	aliases, err := server.GetImageAliases()
	if err != nil {
		return nil, fmt.Errorf("failed to list image aliases: %w", err)
	}
	return aliases, nil
}

// CreateImageAlias creates a new alias for an image
func CreateImageAlias(ctx context.Context, server lxd.InstanceServer, name, fingerprint, description string) error {
	alias := api.ImageAliasesPost{
		ImageAliasesEntry: api.ImageAliasesEntry{
			Name:        name,
			Description: description,
			Target:      fingerprint,
		},
	}

	err := server.CreateImageAlias(alias)
	if err != nil {
		return fmt.Errorf("failed to create image alias %s: %w", name, err)
	}

	return nil
}

// DeleteImageAlias deletes an image alias
func DeleteImageAlias(ctx context.Context, server lxd.InstanceServer, name string) error {
	err := server.DeleteImageAlias(name)
	if err != nil {
		return fmt.Errorf("failed to delete image alias %s: %w", name, err)
	}
	return nil
}

// GetImageFingerprint returns the fingerprint for an image alias or fingerprint
func GetImageFingerprint(ctx context.Context, server lxd.InstanceServer, aliasOrFingerprint string) (string, error) {
	// First try to get it as an alias
	aliasInfo, _, err := server.GetImageAlias(aliasOrFingerprint)
	if err == nil {
		return aliasInfo.Target, nil
	}

	// If not an alias, assume it's a fingerprint and validate
	_, _, err = server.GetImage(aliasOrFingerprint)
	if err != nil {
		return "", fmt.Errorf("image %s not found as alias or fingerprint: %w", aliasOrFingerprint, err)
	}

	return aliasOrFingerprint, nil
}
