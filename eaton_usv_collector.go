package main

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/l3akage/eaton_usv_exporter/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/soniah/gosnmp"
)

const prefix = "eaton_usv_"

var (
	upDesc               *prometheus.Desc
	batteryRemainingDesc *prometheus.Desc
	batteryLevelDesc     *prometheus.Desc
	badInputDesc         *prometheus.Desc
	inputPhaseDesc       *prometheus.Desc
	inputFrequencyDesc   *prometheus.Desc
	outputPhaseDesc      *prometheus.Desc
	outputFrequencyDesc  *prometheus.Desc
	outputLoadDesc       *prometheus.Desc
	outputPowerDesc      *prometheus.Desc
	onBatteryDesc        *prometheus.Desc
	onBypassDesc         *prometheus.Desc
	ambientTemp          *prometheus.Desc
)

func init() {
	l := []string{"target"}
	upDesc = prometheus.NewDesc(prefix+"up", "Scrape of target was successful", l, nil)
	batteryRemainingDesc = prometheus.NewDesc(prefix+"battery_remaining", "The time remaining actual charge vs actual load", l, nil)
	batteryLevelDesc = prometheus.NewDesc(prefix+"battery_charge", "The battery level as a percentage of charge", l, nil)

	badInputDesc = prometheus.NewDesc(prefix+"bad_input", "The utility power bad voltage or bad frequency status. 1=yes,2=no", append(l, "cause"), nil)

	inputPhaseDesc = prometheus.NewDesc(prefix+"input_voltage", "The input phase voltage", append(l, "phase"), nil)
	inputFrequencyDesc = prometheus.NewDesc(prefix+"input_frequency", "The input phase frequency", append(l, "phase"), nil)

	outputPhaseDesc = prometheus.NewDesc(prefix+"output_voltage", "The output phase voltage.", append(l, "phase"), nil)
	outputFrequencyDesc = prometheus.NewDesc(prefix+"output_frequency", "The output phase frequency.", append(l, "phase"), nil)
	outputLoadDesc = prometheus.NewDesc(prefix+"output_load", "The output load.", append(l, "phase"), nil)
	outputPowerDesc = prometheus.NewDesc(prefix+"output_power", "The output power in VA.", append(l, "phase"), nil)

	onBatteryDesc = prometheus.NewDesc(prefix+"on_battery", "The UPS on battery / on main status. 1=yes,2=no", l, nil)
	onBypassDesc = prometheus.NewDesc(prefix+"on_bypass", "The UPS on bypass status. 1=yes,2=no", l, nil)

	ambientTemp = prometheus.NewDesc(prefix+"ambient_temp", "The ambient temperature in the vicinity of the UPS (in degrees C)", l, nil)

	//1.3.6.1.4.1.534.1.4.6.0
}

type eatonUsvCollector struct {
	cfg *config.Config
}

func newEatonUsvCollector(cfg *config.Config) *eatonUsvCollector {
	return &eatonUsvCollector{cfg}
}

func (c eatonUsvCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- upDesc
	ch <- batteryRemainingDesc
	ch <- batteryLevelDesc
	ch <- badInputDesc
	ch <- inputPhaseDesc
	ch <- inputFrequencyDesc
	ch <- outputPhaseDesc
	ch <- outputFrequencyDesc
	ch <- outputLoadDesc
	ch <- outputPowerDesc
	ch <- onBatteryDesc
	ch <- onBypassDesc
	ch <- ambientTemp
}

func badInputStateToText(status int) string {
	switch status {
	case 1:
		return "no"
	case 2:
		return "voltage out of tolerance"
	case 3:
		return "frequency out of tolerance"
	case 4:
		return "no voltage at all"
	}
	return ""
}

func (c eatonUsvCollector) collectInputPhase(snmp *gosnmp.GoSNMP, ch chan<- prometheus.Metric, target string, input int) {
	for i := 1; i <= input; i++ {
		oids := []string{"1.3.6.1.4.1.705.1.6.2.1.2." + strconv.Itoa(i) + ".0", "1.3.6.1.4.1.705.1.6.2.1.3." + strconv.Itoa(i) + ".0"}
		result, err := snmp.Get(oids)
		if err != nil {
			log.Infof("inputPhase Get() err: %v\n", err)
			return
		}
		for _, variable := range result.Variables {
			if variable.Value == nil {
				continue
			}
			switch variable.Name[1:] {
			case oids[0]:
				ch <- prometheus.MustNewConstMetric(inputPhaseDesc, prometheus.GaugeValue, float64(variable.Value.(int)), target, strconv.Itoa(i))
			case oids[1]:
				ch <- prometheus.MustNewConstMetric(inputFrequencyDesc, prometheus.GaugeValue, float64(variable.Value.(int)), target, strconv.Itoa(i))
			}
		}
	}
}

func (c eatonUsvCollector) collectOutputPhase(snmp *gosnmp.GoSNMP, ch chan<- prometheus.Metric, target string, output, power int) {
	for i := 1; i <= output; i++ {
		oids := []string{"1.3.6.1.4.1.705.1.7.2.1.2." + strconv.Itoa(i), "1.3.6.1.4.1.705.1.7.2.1.3." + strconv.Itoa(i), "1.3.6.1.4.1.705.1.7.2.1.4." + strconv.Itoa(i)}
		result, err := snmp.Get(oids)
		if err != nil {
			log.Infof("outputPhase Get() err: %v\n", err)
			return
		}
		for _, variable := range result.Variables {
			if variable.Value == nil {
				continue
			}
			switch variable.Name[1:] {
			case oids[0]:
				ch <- prometheus.MustNewConstMetric(outputPhaseDesc, prometheus.GaugeValue, float64(variable.Value.(int)), target, strconv.Itoa(i))
			case oids[1]:
				ch <- prometheus.MustNewConstMetric(outputFrequencyDesc, prometheus.GaugeValue, float64(variable.Value.(int)), target, strconv.Itoa(i))
			case oids[2]:
				ch <- prometheus.MustNewConstMetric(outputLoadDesc, prometheus.GaugeValue, float64(variable.Value.(int)), target, strconv.Itoa(i))
				ch <- prometheus.MustNewConstMetric(outputPowerDesc, prometheus.GaugeValue, float64((power/100)*variable.Value.(int)), target, strconv.Itoa(i))
			}
		}
	}
}

func (c eatonUsvCollector) collectTarget(target string, ch chan<- prometheus.Metric, wg *sync.WaitGroup) {
	defer wg.Done()
	snmp := &gosnmp.GoSNMP{
		Target:    target,
		Port:      161,
		Community: *snmpCommunity,
		Version:   gosnmp.Version1,
		Timeout:   time.Duration(2) * time.Second,
	}
	err := snmp.Connect()
	if err != nil {
		log.Infof("Connect() err: %v\n", err)
		ch <- prometheus.MustNewConstMetric(upDesc, prometheus.GaugeValue, 0, target)
		return
	}
	defer snmp.Conn.Close()

	oids := []string{"1.3.6.1.4.1.705.1.5.1.0", "1.3.6.1.4.1.705.1.5.2.0", "1.3.6.1.4.1.705.1.6.3.0", "1.3.6.1.4.1.705.1.6.4.0", "1.3.6.1.4.1.705.1.6.1.0"}
	oids = append(oids, "1.3.6.1.4.1.705.1.7.1.0", "1.3.6.1.4.1.705.1.7.3.0", "1.3.6.1.4.1.705.1.7.4.0", "1.3.6.1.4.1.534.1.6.1.0", "1.3.6.1.4.1.705.1.4.12.0")
	result, err2 := snmp.Get(oids)
	if err2 != nil {
		log.Infof("Get() err: %v\n", err2)
		ch <- prometheus.MustNewConstMetric(upDesc, prometheus.GaugeValue, 0, target)
		return
	}
	var inputPhase, outputPhase, badInputStatus, badInputText, power int

	for _, variable := range result.Variables {
		if variable.Value == nil {
			continue
		}
		switch variable.Name[1:] {
		case oids[0]:
			ch <- prometheus.MustNewConstMetric(batteryRemainingDesc, prometheus.GaugeValue, float64(variable.Value.(int)), target)
		case oids[1]:
			ch <- prometheus.MustNewConstMetric(batteryLevelDesc, prometheus.GaugeValue, float64(variable.Value.(int)), target)
		case oids[2]:
			badInputStatus = variable.Value.(int)
		case oids[3]:
			badInputText = variable.Value.(int)
		case oids[4]:
			inputPhase = variable.Value.(int)
		case oids[5]:
			outputPhase = variable.Value.(int)
		case oids[6]:
			ch <- prometheus.MustNewConstMetric(onBatteryDesc, prometheus.GaugeValue, float64(variable.Value.(int)), target)
		case oids[7]:
			ch <- prometheus.MustNewConstMetric(onBypassDesc, prometheus.GaugeValue, float64(variable.Value.(int)), target)
		case oids[8]:
			ch <- prometheus.MustNewConstMetric(ambientTemp, prometheus.GaugeValue, float64(variable.Value.(int)), target)
		case oids[9]:
			power = variable.Value.(int)
		}
	}
	ch <- prometheus.MustNewConstMetric(badInputDesc, prometheus.GaugeValue, float64(badInputStatus), target, badInputStateToText(badInputText))

	c.collectInputPhase(snmp, ch, target, inputPhase)
	c.collectOutputPhase(snmp, ch, target, outputPhase, power)

	ch <- prometheus.MustNewConstMetric(upDesc, prometheus.GaugeValue, 1, target)
}

func (c eatonUsvCollector) Collect(ch chan<- prometheus.Metric) {
	targets := strings.Split(*snmpTargets, ",")
	targets = append(targets, c.cfg.Targets...)
	wg := &sync.WaitGroup{}

	for _, target := range targets {
		wg.Add(1)
		go c.collectTarget(target, ch, wg)
	}

	wg.Wait()
}
