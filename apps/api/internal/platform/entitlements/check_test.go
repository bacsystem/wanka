package entitlements_test

import (
	"testing"

	"github.com/bacsystem/wanka/apps/api/internal/platform/entitlements"
)

func active() entitlements.Entitlements {
	return entitlements.Entitlements{
		Status:  entitlements.StatusActive,
		Modules: []string{"inventario", "ventas"},
		Limits:  map[string]int64{"users": 5, "sites": 2, "cpe_per_month": 100},
		Usage:   map[string]int64{"users": 4, "cpe_per_month": 100},
	}
}

func with(e entitlements.Entitlements, f func(*entitlements.Entitlements)) entitlements.Entitlements {
	f(&e)
	return e
}

// Cada fila fija una regla y el orden entre ellas: gana la primera que falla
// (empresa cerrada, suscripción, módulo, límite).
func TestCheckRulesAndOrder(t *testing.T) {
	inactive := func(s entitlements.Status) func(*entitlements.Entitlements) {
		return func(e *entitlements.Entitlements) { e.Status, e.Modules = s, []string{} }
	}
	closed := func(e *entitlements.Entitlements) { e.ReadOnly = true }
	cases := []struct {
		name string
		e    entitlements.Entitlements
		r    entitlements.Request
		want string // code, "" si pasa
	}{
		{"ok", active(), entitlements.Request{Method: "POST", Module: "ventas"}, ""},
		{"module missing", active(), entitlements.Request{Method: "GET", Module: "rubro.veterinaria"}, entitlements.CodeModuleNotEnabled},
		{"empty module is never enabled", active(), entitlements.Request{Method: "GET"}, entitlements.CodeModuleNotEnabled},
		{"none module passes the module rule", active(), entitlements.Request{Method: "GET", Module: entitlements.NoModule}, ""},
		{"trial is usable", with(active(), func(e *entitlements.Entitlements) { e.Status = entitlements.StatusTrial }), entitlements.Request{Module: "ventas", Method: "GET"}, ""},
		{"past due blocks a GET", with(active(), inactive(entitlements.StatusPastDue)), entitlements.Request{Method: "GET", Module: "ventas"}, entitlements.CodeSubscriptionIdle},
		{"suspended blocks", with(active(), inactive(entitlements.StatusSuspended)), entitlements.Request{Method: "POST", Module: "ventas"}, entitlements.CodeSubscriptionIdle},
		{"cancelled blocks", with(active(), inactive(entitlements.StatusCancelled)), entitlements.Request{Method: "GET", Module: "ventas"}, entitlements.CodeSubscriptionIdle},
		{"inactive: fixed-list route passes", with(active(), inactive(entitlements.StatusPastDue)), entitlements.Request{Method: "GET", Module: entitlements.NoModule, KeepsRunningWhenInactive: true}, ""},
		{"none does not skip the 402", with(active(), inactive(entitlements.StatusPastDue)), entitlements.Request{Method: "GET", Module: entitlements.NoModule}, entitlements.CodeSubscriptionIdle},
		{"no plan is a missing module, not a 402", with(active(), func(e *entitlements.Entitlements) { e.Status, e.Modules = entitlements.StatusNoPlan, []string{} }), entitlements.Request{Method: "GET", Module: "ventas"}, entitlements.CodeModuleNotEnabled},
		{"closed company blocks a write", with(active(), closed), entitlements.Request{Method: "POST", Module: "ventas"}, entitlements.CodeReadOnly},
		{"closed company blocks DELETE", with(active(), closed), entitlements.Request{Method: "DELETE", Module: "ventas"}, entitlements.CodeReadOnly},
		{"closed company blocks PATCH", with(active(), closed), entitlements.Request{Method: "PATCH", Module: "ventas"}, entitlements.CodeReadOnly},
		{"closed company blocks PUT", with(active(), closed), entitlements.Request{Method: "PUT", Module: "ventas"}, entitlements.CodeReadOnly},
		{"closed company reads with HEAD", with(active(), closed), entitlements.Request{Method: "HEAD", Module: "ventas"}, ""},
		{"closed company reads", with(active(), closed), entitlements.Request{Method: "GET", Module: "ventas"}, ""},
		{"none does not skip the read-only rule", with(active(), closed), entitlements.Request{Method: "POST", Module: entitlements.NoModule, KeepsRunningWhenInactive: true}, entitlements.CodeReadOnly},
		{"closed company with a lapsed subscription keeps its last plan's modules: GET passes, not a 402", with(active(), func(e *entitlements.Entitlements) { closed(e); e.Status = entitlements.StatusCancelled }), entitlements.Request{Method: "GET", Module: "ventas"}, ""},
		{"closed company with a lapsed subscription and no module: the module rule decides", with(active(), func(e *entitlements.Entitlements) { closed(e); inactive(entitlements.StatusCancelled)(e) }), entitlements.Request{Method: "GET", Module: "ventas"}, entitlements.CodeModuleNotEnabled},
		{"closed company with a lapsed subscription still reads exports", with(active(), func(e *entitlements.Entitlements) { closed(e); inactive(entitlements.StatusCancelled)(e) }), entitlements.Request{Method: "GET", Module: entitlements.NoModule}, ""},
		{"read-only beats subscription", with(active(), func(e *entitlements.Entitlements) { closed(e); inactive(entitlements.StatusSuspended)(e) }), entitlements.Request{Method: "POST", Module: "ventas"}, entitlements.CodeReadOnly},
		{"subscription beats module", with(active(), inactive(entitlements.StatusPastDue)), entitlements.Request{Method: "POST", Module: "rubro.veterinaria", Limit: "users"}, entitlements.CodeSubscriptionIdle},
		{"module beats limit", with(active(), func(e *entitlements.Entitlements) { e.Usage["users"] = 99 }), entitlements.Request{Method: "POST", Module: "rubro.veterinaria", Limit: "users"}, entitlements.CodeModuleNotEnabled},
		{"limit: used below limit passes", active(), entitlements.Request{Method: "POST", Module: "ventas", Limit: "users"}, ""},
		{"limit: used == limit is reached", with(active(), func(e *entitlements.Entitlements) { e.Usage["users"] = 5 }), entitlements.Request{Method: "POST", Module: "ventas", Limit: "users"}, entitlements.CodePlanLimitReached},
		{"limit: used above limit is reached", with(active(), func(e *entitlements.Entitlements) { e.Usage["users"] = 6 }), entitlements.Request{Method: "POST", Module: "ventas", Limit: "users"}, entitlements.CodePlanLimitReached},
		{"limit: cpe at the edge", active(), entitlements.Request{Method: "POST", Module: "ventas", Limit: "cpe_per_month"}, entitlements.CodePlanLimitReached},
		{"limit: a key missing from the plan closes (limit 0)", with(active(), func(e *entitlements.Entitlements) { delete(e.Limits, "users") }), entitlements.Request{Method: "POST", Module: "ventas", Limit: "users"}, entitlements.CodePlanLimitReached},
		{"limit: nil maps close", entitlements.Entitlements{Status: entitlements.StatusActive, Modules: []string{"ventas"}}, entitlements.Request{Method: "POST", Module: "ventas", Limit: "users"}, entitlements.CodePlanLimitReached},
		{"limit: sites has no usage row, used 0 below limit 2", active(), entitlements.Request{Method: "POST", Module: "ventas", Limit: "sites"}, ""},
		{"a job (no method) of a closed company does not run", with(active(), closed), entitlements.Request{Module: "ventas"}, entitlements.CodeReadOnly},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := c.e.Check(c.r)
			got := ""
			if d != nil {
				got = d.Code
			}
			if got != c.want {
				t.Fatalf("Check = %q, want %q (%+v)", got, c.want, d)
			}
		})
	}
}

func TestCheckCarriesTheContractMembers(t *testing.T) {
	e := with(active(), func(e *entitlements.Entitlements) { e.Usage["users"] = 7 })
	d := e.Check(entitlements.Request{Method: "POST", Module: "ventas", Limit: "users"})
	if d == nil || d.Limit == nil || d.Used == nil || *d.Limit != 5 || *d.Used != 7 || d.Module != "" {
		t.Fatalf("plan_limit_reached = %+v, want limit 5 used 7", d)
	}
	d = e.Check(entitlements.Request{Method: "GET", Module: "rubro.veterinaria"})
	if d == nil || d.Module != "rubro.veterinaria" || d.Limit != nil || d.Used != nil {
		t.Fatalf("module_not_enabled = %+v, want module only", d)
	}
}
