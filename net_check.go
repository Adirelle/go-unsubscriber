package main

import (
	"fmt"
	"github.com/juju/loggo"
	"github.com/yl2chen/cidranger"
	"net"
)

type (
	NonLocalAddressChecker struct {
		cidranger.Ranger
		loggo.Logger
	}
)

func NewNonLocalAddressChecker() (*NonLocalAddressChecker, error) {
	c := &NonLocalAddressChecker{
		cidranger.NewPCTrieRanger(),
		loggo.GetLogger("net_check"),
	}
	if err := c.addLocalNetworks(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *NonLocalAddressChecker) addLocalNetworks() error {
	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("could not list interfaces: %w", err)
	}

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return fmt.Errorf("could not fetch addresses of %q: %w", iface, err)
		}

		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok {
				if err := c.Ranger.Insert(cidranger.NewBasicRangerEntry(*ipNet)); err != nil {
					return fmt.Errorf("could insert %s: %w", ipNet, err)
				} else {
					c.Infof("do not send mail/requests to %s", ipNet)
				}
			}
		}
	}

	return nil
}


func (c *NonLocalAddressChecker) CheckIP(ip net.IP) (bool, error) {
	found, err := c.Ranger.Contains(ip)
	c.Debugf("checked %s: %v (%s)", ip, found, err)
	return err == nil && !found, err
}

func (c *NonLocalAddressChecker) CheckHost(host string) (bool, error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return false, err
	}
	c.Debugf("checking host %q: %v", host, ips)
	for _, ip := range ips {
		if isSafe, err := c.CheckIP(ip); !isSafe || err != nil {
			return false, err
		}
	}
	return true, nil
}

func (c *NonLocalAddressChecker) CheckMX(domain string) (bool, error) {
	mxs, err := net.LookupMX(domain)
	if err != nil {
		return false, err
	}
	for _, mx := range mxs {
		if ok, err := c.CheckHost(mx.Host); !ok || err != nil {
			return false, err
		}
	}
	return true, nil
}
