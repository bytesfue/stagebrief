package gitlab

import "errors"

// ErrNoPreviousPipeline is returned when no successful pipeline exists
// for the branch other than the current one — i.e. this is the first deploy.
var ErrNoPreviousPipeline = errors.New("no previous successful pipeline found")
