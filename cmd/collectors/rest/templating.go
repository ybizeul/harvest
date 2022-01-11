package rest

import (
	"goharvest2/pkg/errors"
	"goharvest2/pkg/matrix"
	"goharvest2/pkg/tree/node"
	"strings"
)

func (r *Rest) initCache() error {

	var (
		counters            *node.Node
		display, name, kind string
		metr                matrix.Metric
		err                 error
	)

	if x := r.Params.GetChildContentS("object"); x != "" {
		r.Object = x
	} else {
		r.Object = strings.ToLower(r.Object)
	}
	r.Matrix.Object = r.Object

	if e := r.Params.GetChildS("export_options"); e != nil {
		r.Matrix.SetExportOptions(e)
	}

	if r.prop.query = r.Params.GetChildContentS("query"); r.prop.query == "" {
		return errors.New(errors.MISSING_PARAM, "query")
	}

	// create metric cache
	if counters = r.Params.GetChildS("counters"); counters == nil {
		return errors.New(errors.MISSING_PARAM, "counters")
	}

	// default value for ONTAP is 15 sec
	if returnTimeout := r.Params.GetChildContentS("return_timeout"); returnTimeout != "" {
		r.prop.returnTimeOut = returnTimeout
	}

	r.prop.instanceKeys = make([]string, 0)
	r.prop.instanceLabels = make(map[string]string)
	r.prop.counters = make(map[string]string)

	for _, c := range counters.GetAllChildContentS() {
		if c != "" {
			name, display, kind = ParseMetric(c)
			r.Logger.Debug().
				Str("kind", kind).
				Str("name", name).
				Str("display", display).
				Msg("Collected")

			r.prop.counters[name] = display
			switch kind {
			case "key":
				r.prop.instanceLabels[name] = display
				r.prop.instanceKeys = append(r.prop.instanceKeys, name)
			case "label":
				r.prop.instanceLabels[name] = display
			case "float":
				if metr, err = r.Matrix.NewMetricFloat64(name); err != nil {
					r.Logger.Error().Err(err).
						Str("name", name).
						Msg("NewMetricFloat64")
					return err
				}
				metr.SetName(display)
				metr.SetProperty("etl.float")
			}
		}
	}

	r.Logger.Info().Strs("extracted Instance Keys", r.prop.instanceKeys).Msg("")
	r.Logger.Info().Int("count metrics", len(r.Matrix.GetMetrics())).Int("count labels", len(r.prop.instanceLabels)).Msg("initialized metric cache")

	if len(r.Matrix.GetMetrics()) == 0 && r.Params.GetChildContentS("collect_only_labels") != "true" {
		return errors.New(errors.ERR_NO_METRIC, "failed to parse numeric metrics")
	}
	return nil
}

func ParseMetric(rawName string) (string, string, string) {
	var (
		name, display string
		values        []string
	)
	if values = strings.SplitN(rawName, "=>", 2); len(values) == 2 {
		name = strings.TrimSpace(values[0])
		display = strings.TrimSpace(values[1])
	} else {
		name = rawName
		display = strings.ReplaceAll(rawName, ".", "_")
	}

	if strings.HasPrefix(name, "^^") {
		return strings.TrimPrefix(name, "^^"), strings.TrimPrefix(display, "^^"), "key"
	}

	if strings.HasPrefix(name, "^") {
		return strings.TrimPrefix(name, "^"), strings.TrimPrefix(display, "^"), "label"
	}

	if strings.HasPrefix(name, "?") {
		return strings.TrimPrefix(name, "?"), strings.TrimPrefix(display, "?"), "bool"
	}

	return name, display, "float"
}
