package devcontainer

import (
	"context"
	"fmt"
)

// ResolveImage resolves the roost image and materialize dir for projectPath.
// Priority: roost-proj-<hash> (project-scope) → roost-user (user-scope).
// Returns an error if neither image exists.
func ResolveImage(ctx context.Context, projectPath string) (image, materializeDir string, err error) {
	hash := projectHash(projectPath)

	projImage := ProjectScopeImage(hash)
	ok, err := ImageExists(ctx, projImage)
	if err != nil {
		return "", "", fmt.Errorf("check project image: %w", err)
	}
	if ok {
		return projImage, ProjectMaterializeDir(hash), nil
	}

	userImage := UserScopeImage()
	ok, err = ImageExists(ctx, userImage)
	if err != nil {
		return "", "", fmt.Errorf("check user image: %w", err)
	}
	if ok {
		return userImage, UserMaterializeDir(), nil
	}

	return "", "", fmt.Errorf("no roost image found for %s; run 'roost build %s' or 'roost build --user' first", projectPath, projectPath)
}
