package gw_cache

type DefaultReporter struct{}

func (d *DefaultReporter) ReportMiss() {}

func (d *DefaultReporter) ReportHit() {}
