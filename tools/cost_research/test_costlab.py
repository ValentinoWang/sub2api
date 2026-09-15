import datetime as dt
from decimal import Decimal
import unittest
from costlab import radar_parse, vnstat_parse, traffic_cost, reset_gain, seven_day_mean, two_month_rate, private_projection, forecast


class CostTests(unittest.TestCase):
    def test_public_snapshot_is_not_history(self):
        row = radar_parse('<h2>Quota Radar 8月9日19:49更新</h2><p>20x Pro $1,649.72 Distributed Radar</p><p>Plus $82.49 estimated</p><h2>Fast Radar</h2>')
        self.assertEqual(row['weekly_quota'][0]['weekly_reference_usd'], 1649.72)
        self.assertIsNone(row['seven_day_average'])
        self.assertFalse(row['history_complete'])
        self.assertIsNone(row['requested_models']['gpt-6-astra'])

    def test_dynamic_shell_fails_closed(self):
        self.assertEqual(radar_parse('<body>Loading...</body>')['status'], 'DYNAMIC_OR_SCHEMA_CHANGED')

    def test_refill_is_incremental(self):
        self.assertAlmostEqual(reset_gain(.8, 1), .2)
        self.assertEqual(reset_gain(1, 1), 0)
        self.assertAlmostEqual(reset_gain(.8, .95, .05), .2)

    def test_percent_is_not_fraction(self):
        with self.assertRaises(ValueError): reset_gain(80, 100)

    def test_included_traffic_once_per_cycle(self):
        self.assertEqual(traffic_cost(100*10**9, 150*10**9, 100*10**9, 'egress', '.1'), Decimal('5.0'))
        self.assertEqual(traffic_cost(100*10**9, 150*10**9, 100*10**9, 'both', '.1'), Decimal('15.0'))

    def test_95th_is_not_gb_tariff(self):
        with self.assertRaises(ValueError): traffic_cost(0, 10, 0, '95th', '.1')

    def test_nan_tariff(self):
        with self.assertRaises(ValueError): traffic_cost(0, 10, 0, 'egress', 'NaN')

    def test_vnstat_units_and_interface(self):
        data = {'jsonversion': '2', 'interfaces': [{'name': 'eth0', 'traffic': {'total': {'rx': 1, 'tx': 2}}}]}
        self.assertEqual(vnstat_parse(data, 'eth0')['unit'], 'byte')
        with self.assertRaises(ValueError): vnstat_parse(data, 'veth0')
        data['jsonversion'] = '1'
        with self.assertRaises(ValueError): vnstat_parse(data, 'eth0')

    def test_average_requires_seven_comparable_days(self):
        rows = [{'date': str(dt.date(2026,9,d)), 'value': 100, 'pool': 'p', 'model': 'm', 'price_version': 'v'} for d in range(9,16)]
        self.assertEqual(seven_day_mean(rows, '2026-09-16')['value'], 100)
        self.assertIsNone(seven_day_mean(rows[:-1], '2026-09-16')['value'])
        rows[0]['price_version'] = 'v2'
        self.assertIsNone(seven_day_mean(rows, '2026-09-16')['value'])
        rows[0]['price_version'] = None
        self.assertIsNone(seven_day_mean(rows, '2026-09-16')['value'])

    def test_two_calendar_months_and_dedup(self):
        event = {'event_id': 'e', 'date': '2026-07-16', 'kind': 'direct_reset_applied', 'verified': True}
        end = dict(event, event_id='end', date='2026-09-16')
        grant = dict(event, event_id='card', kind='card_granted')
        result = two_month_rate([event, event, end, grant], '2026-09-16', True)
        self.assertEqual(result['events'], 1)
        self.assertEqual(result['coverage_days'], 62)
        self.assertIsNone(two_month_rate([event], '2026-09-16', False)['per_week'])

    def test_private_projection(self):
        data = {'data': {'email': 'private', 'credentials': {'token': 'secret'}, 'plan_type': 'pro', 'rate_limit': None}}
        self.assertEqual(private_projection(data, 'quota'), {'plan_type': 'pro', 'rate_limit': None})

    def test_conditional_posterior_reproducible(self):
        cfg = {'own_exposure_complete': True, 'weekly_reference_capacity': 1000, 'billing_days': 30,
               'cash_and_allocated_cost': 200, 'prior_event_shape': 1, 'prior_exposure_weeks': 1,
               'own_observed_weeks': 4, 'own_event_count': 4, 'verified_net_refill_fractions': [.2,.4,.6,.8],
               'useful_utilization': .5, 'seed': 7}
        x = forecast(cfg)
        self.assertEqual(x, forecast(cfg))
        self.assertLessEqual(x['cost_p10'], x['cost_p50'])
        self.assertLessEqual(x['cost_p50'], x['cost_p90'])
        self.assertEqual(x['event_rate_posterior_mean_per_week'], 1)
        cfg['own_exposure_complete'] = False
        with self.assertRaises(ValueError): forecast(cfg)

if __name__ == '__main__': unittest.main()
