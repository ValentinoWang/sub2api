package service

import "testing"

// Lifecycle emails are only reachable from the admin template UI when they appear in the
// ordered event list; being present in the definitions/templates maps alone is not enough.
func TestLifecycleEmailEventsAreAdminVisible(t *testing.T) {
	lifecycle := []string{
		NotificationEmailEventUserWelcome,
		NotificationEmailEventUserInactive,
		NotificationEmailEventUserWinback,
	}

	ordered := make(map[string]bool, len(notificationEmailEventOrder))
	for _, event := range notificationEmailEventOrder {
		ordered[event] = true
	}

	for _, event := range lifecycle {
		if !ordered[event] {
			t.Errorf("%s missing from notificationEmailEventOrder: it would not appear in the admin template list", event)
		}
		info, ok := notificationEmailEventDefinitions[event]
		if !ok {
			t.Errorf("%s missing from notificationEmailEventDefinitions", event)
			continue
		}
		if !info.Optional {
			t.Errorf("%s must be optional so recipients can unsubscribe", event)
		}
		templates, ok := notificationEmailOfficialTemplates[event]
		if !ok {
			t.Errorf("%s missing from notificationEmailOfficialTemplates", event)
			continue
		}
		for _, locale := range []string{notificationEmailDefaultLocale, notificationEmailLocaleChinese} {
			tpl, ok := templates[locale]
			if !ok || tpl.Subject == "" || tpl.HTML == "" {
				t.Errorf("%s is missing a %s template", event, locale)
			}
		}
	}
}

// Every ordered event must resolve, otherwise the admin list renders an entry that errors on open.
func TestEveryOrderedEmailEventHasDefinitionAndTemplate(t *testing.T) {
	for _, event := range notificationEmailEventOrder {
		if _, ok := notificationEmailEventDefinitions[event]; !ok {
			t.Errorf("ordered event %s has no definition", event)
		}
		if _, ok := notificationEmailOfficialTemplates[event]; !ok {
			t.Errorf("ordered event %s has no official template", event)
		}
	}
}
