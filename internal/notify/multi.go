package notify

import (
	"context"
	"fmt"

	"github.com/BrunoTulio/logr"
)

type MultiNotifier struct {
	notifiers      []Notifier
	successEnabled bool
	errorEnabled   bool
	log            logr.Logger
}

func (m *MultiNotifier) AddNotifier(notifier Notifier) {
	m.notifiers = append(m.notifiers, notifier)
}

func (m *MultiNotifier) Success(ctx context.Context, msg string) error {
	if !m.successEnabled {
		return nil
	}

	var errs []error

	for _, notifier := range m.notifiers {
		if err := notifier.Success(ctx, msg); err != nil {
			errs = append(errs, err)
			m.log.Warnf("Notifier sendSuccess failed: %v", err)
		}
	}

	if len(errs) == len(m.notifiers) {
		return fmt.Errorf("all notifiers failed: %v", errs)
	}
	return nil
}

func (m *MultiNotifier) Error(ctx context.Context, errMsg string) error {

	if !m.errorEnabled {
		return nil
	}

	var errs []error
	for _, n := range m.notifiers {
		if err := n.Error(ctx, errMsg); err != nil {
			errs = append(errs, err)
			m.log.Warnf("Notifier sendError failed: %v", err)
		}
	}
	if len(errs) == len(m.notifiers) {
		return fmt.Errorf("all notifiers failed: %v", errs)
	}

	return nil
}

func NewMultiNotifier(
	enablesSuccess bool,
	enabledError bool,
	log logr.Logger,
) *MultiNotifier {
	return &MultiNotifier{
		successEnabled: enablesSuccess,
		errorEnabled:   enabledError,
		log:            log,
	}
}
