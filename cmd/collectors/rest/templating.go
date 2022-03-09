package rest

import (
	"fmt"
	"goharvest2/pkg/errors"
	"goharvest2/pkg/tree/node"
	"goharvest2/pkg/util"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func (r *Rest) LoadTemplate() (string, error) {

	var (
		template     *node.Node
		templatePath string
		err          error
	)

	// import template
	if template, templatePath, err = r.ImportSubTemplate("", r.getTemplateFn(), r.Client.Cluster().Version); err != nil {
		return "", err
	}

	r.Params.Union(template)
	return templatePath, nil
}

func (r *Rest) InitCache() error {

	var (
		counters *node.Node
	)

	if x := r.Params.GetChildContentS("object"); x != "" {
		r.Prop.Object = x
	} else {
		r.Prop.Object = strings.ToLower(r.Object)
	}

	if e := r.Params.GetChildS("export_options"); e != nil {
		r.Matrix.SetExportOptions(e)
	}

	if r.Prop.Query = r.Params.GetChildContentS("query"); r.Prop.Query == "" {
		return errors.New(errors.MISSING_PARAM, "query")
	}

	// create metric cache
	if counters = r.Params.GetChildS("counters"); counters == nil {
		return errors.New(errors.MISSING_PARAM, "counters")
	}

	// default value for ONTAP is 15 sec
	if returnTimeout := r.Params.GetChildContentS("return_timeout"); returnTimeout != "" {
		r.Prop.ReturnTimeOut = returnTimeout
	}

	r.Prop.InstanceKeys = make([]string, 0)
	r.Prop.InstanceLabels = make(map[string]string)
	r.Prop.Counters = make(map[string]string)
	r.Prop.Metrics = make(map[string]*Metric)

	// private end point do not support * as fields. We need to pass fields in endpoint
	query := r.Params.GetChildS("query")
	r.Prop.ApiType = "public"
	if query != nil {
		r.Prop.ApiType = checkQueryType(query.GetContentS())
	}

	r.ParseRestCounters(counters, r.Prop)
	r.Metadata.NewMetricUint64("datapoint_count")

	r.Logger.Info().Strs("extracted Instance Keys", r.Prop.InstanceKeys).Msg("")
	r.Logger.Info().Int("count metrics", len(r.Prop.Metrics)).Int("count labels", len(r.Prop.InstanceLabels)).Msg("initialized metric cache")

	return nil
}

func HandleDuration(value string) float64 {
	// Example: duration: PT8H35M42S
	timeDurationRegex := `^P(?:(\d+)Y)?(?:(\d+)M)?(?:(\d+)D)?T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+(?:.\d+)?)S)?$`

	regexTimeDuration := regexp.MustCompile(timeDurationRegex)
	if match := regexTimeDuration.MatchString(value); match {
		// example: PT8H35M42S   ==>  30942
		matches := regexTimeDuration.FindStringSubmatch(value)
		if matches == nil {
			return 0
		}

		seconds := 0.0

		//years
		//months

		//days
		if matches[3] != "" {
			f, err := strconv.ParseFloat(matches[3], 64)
			if err != nil {
				fmt.Printf("%v", err)
				return 0
			}
			seconds += f * 24 * 60 * 60
		}

		//hours
		if matches[4] != "" {
			f, err := strconv.ParseFloat(matches[4], 64)
			if err != nil {
				fmt.Printf("%v", err)
				return 0
			}
			seconds += f * 60 * 60
		}

		//minutes
		if matches[5] != "" {
			f, err := strconv.ParseFloat(matches[5], 64)
			if err != nil {
				fmt.Printf("%v", err)
				return 0
			}
			seconds += f * 60
		}

		//seconds & milliseconds
		if matches[6] != "" {
			f, err := strconv.ParseFloat(matches[6], 64)
			if err != nil {
				fmt.Printf("%v", err)
				return 0
			}
			seconds += f
		}
		return seconds
	}

	return 0
}

func HandleTimestamp(value string) float64 {
	var timestamp time.Time
	var err error

	// Example: timestamp: 2020-12-02T18:36:19-08:00
	timestampRegex := `[+-]?\d{4}(-[01]\d(-[0-3]\d(T[0-2]\d:[0-5]\d:?([0-5]\d(\.\d+)?)?[+-][0-2]\d:[0-5]\d?)?)?)?`

	regexTimeStamp := regexp.MustCompile(timestampRegex)
	if match := regexTimeStamp.MatchString(value); match {
		// example: 2020-12-02T18:36:19-08:00   ==>  1606962979
		if timestamp, err = time.Parse(time.RFC3339, value); err != nil {
			fmt.Printf("%v", err)
			return 0
		}
		return float64(timestamp.Unix())
	}
	return 0
}

func (r *Rest) ParseRestCounters(counter *node.Node, prop *prop) {
	var (
		display, name, kind, metricType string
	)

	for _, c := range counter.GetAllChildContentS() {
		if c != "" {
			name, display, kind, metricType = util.ParseMetric(c)
			r.Logger.Debug().
				Str("kind", kind).
				Str("name", name).
				Str("display", display).
				Msg("Collected")

			prop.Counters[name] = display
			switch kind {
			case "key":
				prop.InstanceLabels[name] = display
				prop.InstanceKeys = append(prop.InstanceKeys, name)
			case "label":
				prop.InstanceLabels[name] = display
			case "float":
				m := &Metric{Label: display, Name: name, MetricType: metricType, Exportable: true}
				prop.Metrics[name] = m
			}
		}
	}

	if prop.ApiType == "private" {
		counterKey := make([]string, len(prop.Counters))
		i := 0
		for k := range prop.Counters {
			counterKey[i] = k
			i++
		}
		prop.Fields = counterKey
	}

	if prop.ApiType == "public" {
		prop.Fields = []string{"*"}
		if counter != nil {
			if x := counter.GetChildS("hidden_fields"); x != nil {
				prop.Fields = append(prop.Fields, x.GetAllChildContentS()...)
			}
		}
	}

}
