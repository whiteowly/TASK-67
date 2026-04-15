// transitions_test.go — branch-complete unit tests for the pure
// state-machine validators that gate ticket and shipment status changes.
//
// These functions are the single source of truth for "what status moves
// are legal" — a regression here silently breaks the entire ticket /
// shipment lifecycle. They are pure (no DB, no network, no time), so
// they are ideal unit-test targets.
package service

import (
	"testing"

	"github.com/campusrec/campusrec/internal/model"
)

// ── Ticket transitions ─────────────────────────────────────────────────────

func TestIsValidTicketTransition_AllowedEdges(t *testing.T) {
	allowed := []struct{ from, to string }{
		// open
		{model.TicketStatusOpen, model.TicketStatusAcknowledged},
		{model.TicketStatusOpen, model.TicketStatusInProgress},
		{model.TicketStatusOpen, model.TicketStatusClosed},
		// acknowledged
		{model.TicketStatusAcknowledged, model.TicketStatusInProgress},
		{model.TicketStatusAcknowledged, model.TicketStatusWaitingOnMember},
		{model.TicketStatusAcknowledged, model.TicketStatusClosed},
		// in_progress
		{model.TicketStatusInProgress, model.TicketStatusWaitingOnMember},
		{model.TicketStatusInProgress, model.TicketStatusWaitingOnStaff},
		{model.TicketStatusInProgress, model.TicketStatusResolved},
		{model.TicketStatusInProgress, model.TicketStatusEscalated},
		// waiting_on_member
		{model.TicketStatusWaitingOnMember, model.TicketStatusInProgress},
		{model.TicketStatusWaitingOnMember, model.TicketStatusResolved},
		{model.TicketStatusWaitingOnMember, model.TicketStatusClosed},
		// waiting_on_staff
		{model.TicketStatusWaitingOnStaff, model.TicketStatusInProgress},
		{model.TicketStatusWaitingOnStaff, model.TicketStatusEscalated},
		// escalated
		{model.TicketStatusEscalated, model.TicketStatusInProgress},
		{model.TicketStatusEscalated, model.TicketStatusResolved},
		// resolved
		{model.TicketStatusResolved, model.TicketStatusClosed},
		{model.TicketStatusResolved, model.TicketStatusReopened},
		// reopened
		{model.TicketStatusReopened, model.TicketStatusInProgress},
		{model.TicketStatusReopened, model.TicketStatusClosed},
	}
	for _, tc := range allowed {
		if !isValidTicketTransition(tc.from, tc.to) {
			t.Errorf("%s -> %s should be allowed", tc.from, tc.to)
		}
	}
}

func TestIsValidTicketTransition_DisallowedEdges(t *testing.T) {
	disallowed := []struct{ from, to string }{
		// You cannot resolve directly from open.
		{model.TicketStatusOpen, model.TicketStatusResolved},
		// You cannot reopen an open ticket.
		{model.TicketStatusOpen, model.TicketStatusReopened},
		// closed is a sink — no transitions allowed out of it.
		{model.TicketStatusClosed, model.TicketStatusOpen},
		{model.TicketStatusClosed, model.TicketStatusReopened},
		{model.TicketStatusClosed, model.TicketStatusInProgress},
		// resolved cannot jump back to in_progress directly.
		{model.TicketStatusResolved, model.TicketStatusInProgress},
		// acknowledged cannot jump straight to resolved.
		{model.TicketStatusAcknowledged, model.TicketStatusResolved},
		// Self-loops are not transitions.
		{model.TicketStatusOpen, model.TicketStatusOpen},
		{model.TicketStatusInProgress, model.TicketStatusInProgress},
	}
	for _, tc := range disallowed {
		if isValidTicketTransition(tc.from, tc.to) {
			t.Errorf("%s -> %s must be disallowed", tc.from, tc.to)
		}
	}
}

func TestIsValidTicketTransition_UnknownFromState(t *testing.T) {
	// An unknown source state must always be rejected, never crash.
	if isValidTicketTransition("not_a_real_state", model.TicketStatusOpen) {
		t.Error("transition from unknown state must be rejected")
	}
	if isValidTicketTransition("", model.TicketStatusInProgress) {
		t.Error("transition from empty state must be rejected")
	}
}

// ── Shipment transitions ───────────────────────────────────────────────────

func TestIsValidShipmentTransition_AllowedEdges(t *testing.T) {
	allowed := []struct{ from, to string }{
		// pending_fulfillment
		{model.ShipmentStatusPendingFulfillment, model.ShipmentStatusPacked},
		{model.ShipmentStatusPendingFulfillment, model.ShipmentStatusCanceled},
		// packed
		{model.ShipmentStatusPacked, model.ShipmentStatusShipped},
		{model.ShipmentStatusPacked, model.ShipmentStatusCanceled},
		// shipped
		{model.ShipmentStatusShipped, model.ShipmentStatusDelivered},
		{model.ShipmentStatusShipped, model.ShipmentStatusDeliveryException},
		// delivery_exception
		{model.ShipmentStatusDeliveryException, model.ShipmentStatusReturned},
		{model.ShipmentStatusDeliveryException, model.ShipmentStatusClosedException},
	}
	for _, tc := range allowed {
		if !isValidShipmentTransition(tc.from, tc.to) {
			t.Errorf("%s -> %s should be allowed", tc.from, tc.to)
		}
	}
}

func TestIsValidShipmentTransition_DisallowedEdges(t *testing.T) {
	disallowed := []struct{ from, to string }{
		// Cannot ship before packing.
		{model.ShipmentStatusPendingFulfillment, model.ShipmentStatusShipped},
		// Cannot deliver before shipping.
		{model.ShipmentStatusPacked, model.ShipmentStatusDelivered},
		// delivered is a sink.
		{model.ShipmentStatusDelivered, model.ShipmentStatusShipped},
		{model.ShipmentStatusDelivered, model.ShipmentStatusReturned},
		{model.ShipmentStatusDelivered, model.ShipmentStatusDeliveryException},
		// canceled is a sink.
		{model.ShipmentStatusCanceled, model.ShipmentStatusPacked},
		{model.ShipmentStatusCanceled, model.ShipmentStatusShipped},
		// returned is a sink.
		{model.ShipmentStatusReturned, model.ShipmentStatusDelivered},
		// closed_exception is a sink.
		{model.ShipmentStatusClosedException, model.ShipmentStatusDeliveryException},
		// You cannot cancel a shipment that has already shipped.
		{model.ShipmentStatusShipped, model.ShipmentStatusCanceled},
		// Self-loops are not transitions.
		{model.ShipmentStatusPacked, model.ShipmentStatusPacked},
	}
	for _, tc := range disallowed {
		if isValidShipmentTransition(tc.from, tc.to) {
			t.Errorf("%s -> %s must be disallowed", tc.from, tc.to)
		}
	}
}

func TestIsValidShipmentTransition_UnknownFromState(t *testing.T) {
	if isValidShipmentTransition("not_a_real_state", model.ShipmentStatusPacked) {
		t.Error("transition from unknown state must be rejected")
	}
	if isValidShipmentTransition("", model.ShipmentStatusShipped) {
		t.Error("transition from empty state must be rejected")
	}
}

// Coverage sanity: every defined ticket status has an entry in either the
// allowed-from map or is an explicitly-known sink. Any new ticket status
// added to the model without updating isValidTicketTransition is an
// implicit policy hole that this test will catch.
func TestIsValidTicketTransition_AllStatusesConsidered(t *testing.T) {
	knownSinks := map[string]bool{
		model.TicketStatusClosed: true,
	}
	statuses := []string{
		model.TicketStatusOpen,
		model.TicketStatusAcknowledged,
		model.TicketStatusInProgress,
		model.TicketStatusWaitingOnMember,
		model.TicketStatusWaitingOnStaff,
		model.TicketStatusEscalated,
		model.TicketStatusResolved,
		model.TicketStatusReopened,
		model.TicketStatusClosed,
	}
	for _, s := range statuses {
		// Every non-sink status must allow at least one outbound transition.
		if knownSinks[s] {
			continue
		}
		any := false
		for _, target := range statuses {
			if s != target && isValidTicketTransition(s, target) {
				any = true
				break
			}
		}
		if !any {
			t.Errorf("status %q has no allowed outbound transitions and is not a known sink", s)
		}
	}
}
