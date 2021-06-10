package main

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/l3akage/eaton_usv_exporter/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const prefix = "eaton_usv_"

var (
	upDesc               *prometheus.Desc
	batteryRemainingDesc *prometheus.Desc
	batteryLevelDesc     *prometheus.Desc
	inputPhaseDesc       *prometheus.Desc
	inputFrequencyDesc   *prometheus.Desc
	outputPhaseDesc      *prometheus.Desc
	outputFrequencyDesc  *prometheus.Desc
	outputLoadDesc       *prometheus.Desc
	outputPowerDesc      *prometheus.Desc
	ambientTemp          *prometheus.Desc
)

func init() {
	l := []string{"target"}
	upDesc = prometheus.NewDesc(prefix+"up", "Scrape of target was successful", l, nil)
	batteryRemainingDesc = prometheus.NewDesc(prefix+"battery_remaining", "The time remaining actual charge vs actual load", l, nil)
	batteryLevelDesc = prometheus.NewDesc(prefix+"battery_charge", "The battery level as a percentage of charge", l, nil)
	outputFrequencyDesc = prometheus.NewDesc(prefix+"output_frequency", "The output frequency.", l, nil)
	inputFrequencyDesc = prometheus.NewDesc(prefix+"input_frequency", "The input frequency", l, nil)
	outputLoadDesc = prometheus.NewDesc(prefix+"output_load", "The output load.", l, nil)
	outputPowerDesc = prometheus.NewDesc(prefix+"output_power", "The output power in VA.", l, nil)

	inputPhaseDesc = prometheus.NewDesc(prefix+"input_voltage", "The input phase voltage", append(l, "phase"), nil)
	outputPhaseDesc = prometheus.NewDesc(prefix+"output_voltage", "The output phase voltage.", append(l, "phase"), nil)

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
	ch <- inputPhaseDesc
	ch <- inputFrequencyDesc
	ch <- outputPhaseDesc
	ch <- outputFrequencyDesc
	ch <- outputLoadDesc
	ch <- outputPowerDesc
	ch <- ambientTemp
}

func (c eatonUsvCollector) collectInputPhase(snmp *gosnmp.GoSNMP, ch chan<- prometheus.Metric, target string, input int) {
	for i := 1; i <= input; i++ {
		oids := []string{"1.3.6.1.4.1.534.1.3.4." + strconv.Itoa(i) + ".2.1", "1.3.6.1.4.1.534.1.3.4." + strconv.Itoa(i) + ".2.1.0"}
		for _, oid := range oids {
			result, err := snmp.Get([]string{oid})
			if err != nil {
				if *debug {
					log.Infof("inputPhase Get() err: %v\n", err)
				}
				return
			}

			if result.Error == gosnmp.NoSuchName && snmp.Version == gosnmp.Version1 {
				if *debug {
					log.Infof("NoSuchName: %s\n", oid)
				}
				continue
			}

			for _, v := range result.Variables {
				if v.Value == nil {
					continue
				}
				switch v.Name[1:] {
				case oids[0], oids[1]:
					ch <- prometheus.MustNewConstMetric(inputPhaseDesc, prometheus.GaugeValue, float64(v.Value.(int)), target, strconv.Itoa(i))
				}
			}
		}
	}
}

func (c eatonUsvCollector) collectOutputPhase(snmp *gosnmp.GoSNMP, ch chan<- prometheus.Metric, target string, output int) {
	for i := 1; i <= output; i++ {
		oids := []string{"1.3.6.1.4.1.534.1.4.4." + strconv.Itoa(i) + ".2.1", "1.3.6.1.4.1.534.1.4.4." + strconv.Itoa(i) + ".2.1.0"}
		for _, oid := range oids {
			result, err := snmp.Get([]string{oid})
			if err != nil {
				if *debug {
					log.Infof("outputPhase Get() err: %v\n", err)
				}
				return
			}

			if result.Error == gosnmp.NoSuchName && snmp.Version == gosnmp.Version1 {
				if *debug {
					log.Infof("NoSuchName: %s\n", oid)
				}
				continue
			}

			for _, v := range result.Variables {
				if v.Value == nil {
					continue
				}
				switch v.Name[1:] {
				case oids[0], oids[1]:
					ch <- prometheus.MustNewConstMetric(outputPhaseDesc, prometheus.GaugeValue, float64(v.Value.(int)), target, strconv.Itoa(i))
				}
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
		if *debug {
			log.Infof("Connect() err: %v\n", err)
		}
		ch <- prometheus.MustNewConstMetric(upDesc, prometheus.GaugeValue, 0, target)
		return
	}
	defer snmp.Conn.Close()

	oids := []string{"1.3.6.1.4.1.534.1.2.1.0", "1.3.6.1.4.1.534.1.2.4.0", "1.3.6.1.4.1.534.1.3.3.0", "1.3.6.1.4.1.534.1.4.3.0"}
	oids = append(oids, "1.3.6.1.4.1.534.1.6.1.0", "1.3.6.1.4.1.534.1.4.2.0", "1.3.6.1.4.1.534.1.3.1.0", "1.3.6.1.4.1.534.1.4.1.0", "1.3.6.1.4.1.534.1.10.3.0")
	result, err2 := snmp.Get(oids)
	if err2 != nil {
		if *debug {
			log.Infof("Get() err: %v from %s\n", err2, target)
		}
		ch <- prometheus.MustNewConstMetric(upDesc, prometheus.GaugeValue, 0, target)
		return
	}
	var inputPhase, outputPhase, power, load int

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
			inputPhase = variable.Value.(int)
		case oids[3]:
			outputPhase = variable.Value.(int)
		case oids[4]:
			ch <- prometheus.MustNewConstMetric(ambientTemp, prometheus.GaugeValue, float64(variable.Value.(int)), target)
		case oids[5]:
			ch <- prometheus.MustNewConstMetric(outputFrequencyDesc, prometheus.GaugeValue, float64(variable.Value.(int)), target)
		case oids[6]:
			ch <- prometheus.MustNewConstMetric(inputFrequencyDesc, prometheus.GaugeValue, float64(variable.Value.(int)), target)
		case oids[7]:
			ch <- prometheus.MustNewConstMetric(outputLoadDesc, prometheus.GaugeValue, float64(variable.Value.(int)), target)
			load = variable.Value.(int)
		case oids[8]:
			power = variable.Value.(int)
		}
	}

	c.collectInputPhase(snmp, ch, target, inputPhase)
	c.collectOutputPhase(snmp, ch, target, outputPhase)

	ch <- prometheus.MustNewConstMetric(outputPowerDesc, prometheus.GaugeValue, float64((power/100)*load), target)

	ch <- prometheus.MustNewConstMetric(upDesc, prometheus.GaugeValue, 1, target)
}

func (c eatonUsvCollector) Collect(ch chan<- prometheus.Metric) {
	targets := strings.Split(*snmpTargets, ",")
	targets = append(targets, c.cfg.Targets...)
	wg := &sync.WaitGroup{}

	for _, target := range targets {
		if target == "" {
			continue
		}
		wg.Add(1)
		go c.collectTarget(target, ch, wg)
	}

	wg.Wait()
}
