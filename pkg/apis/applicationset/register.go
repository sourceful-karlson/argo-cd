package applicationset

const (
	// API Group
	Group string = "appset.argoproj.io"

	// ApplicationSet constants
	ApplicationSetKind      string = "Applicationset"
	ApplicationSetSingular  string = "applicationset"
	ApplicationSetShortName string = "appset"
	ApplicationSetPlural    string = "applicationsets"
	ApplicationSetFullName  string = ApplicationSetPlural + "." + Group
)
