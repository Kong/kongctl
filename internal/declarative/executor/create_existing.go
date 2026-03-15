package executor

import decerrors "github.com/kong/kongctl/internal/declarative/errors"

const existingCreateReason = "resource already exists"

func classifyCreateExistingError(err error) (string, bool) {
	statusCode := decerrors.ExtractStatusCodeFromError(err)
	if !decerrors.IsAlreadyExistsError(err, statusCode) {
		return "", false
	}

	return existingCreateReason, true
}
