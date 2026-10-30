import json
import unittest
from unittest.mock import Mock, patch

import ssm_proxy


class SessionTests(unittest.TestCase):
    def test_ssh_stream_and_session_cleanup(self):
        client = Mock()
        client.meta.endpoint_url = "https://ssm.us-east-1.amazonaws.com"
        client.start_session.return_value = {"SessionId": "test-session"}
        with patch.object(ssm_proxy.boto3, "client", return_value=client), patch.object(
            ssm_proxy.subprocess, "call", return_value=7
        ) as plugin:
            self.assertEqual(ssm_proxy.connect("i-0123456789abcdef0", "22", "us-east-1"), 7)
        parameters = client.start_session.call_args.kwargs
        self.assertEqual(parameters["DocumentName"], "AWS-StartSSHSession")
        self.assertEqual(parameters["Parameters"], {"portNumber": ["22"]})
        self.assertEqual(json.loads(plugin.call_args.args[0][5]), parameters)
        self.assertEqual(plugin.call_args.kwargs, {})  # Preserve the SSH byte stream.
        client.terminate_session.assert_called_once_with(SessionId="test-session")

    def test_cleanup_if_plugin_cannot_start(self):
        client = Mock()
        client.start_session.return_value = {"SessionId": "test-session"}
        with patch.object(ssm_proxy.boto3, "client", return_value=client), patch.object(
            ssm_proxy.subprocess, "call", side_effect=OSError("unavailable")
        ), self.assertRaises(OSError):
            ssm_proxy.connect("i-0123456789abcdef0", "22", "us-east-1")
        client.terminate_session.assert_called_once_with(SessionId="test-session")

    def test_rejects_other_destinations(self):
        with patch.object(ssm_proxy.boto3, "client") as client:
            for host, port in [("10.0.0.1", "22"), ("i-0123456789abcdef0", "80")]:
                with self.assertRaises(ValueError):
                    ssm_proxy.connect(host, port, "us-east-1")
            client.assert_not_called()
