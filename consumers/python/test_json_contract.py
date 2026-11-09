"""Consumer-Vertragstests: prüfen die echte distfit-Ausgabe aus Sicht eines Python-Werkzeugs."""

import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import jsonschema

sys.path.insert(0, str(Path(__file__).resolve().parent))

from distfit_client import FitReport, run_distfit  # noqa: E402

REPO = Path(__file__).resolve().parents[2]
SCHEMA = json.loads((REPO / "docs" / "json.schema.json").read_text(encoding="utf-8"))
WINE = str(REPO / "testdata" / "winequality-red.csv")


class DistfitContractTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls._tmp = tempfile.TemporaryDirectory()
        cls.binary = os.environ.get("DISTFIT_BIN")
        if not cls.binary:
            cls.binary = os.path.join(cls._tmp.name, "distfit")
            subprocess.run(
                ["go", "build", "-o", cls.binary, "./cmd/distfit"],
                cwd=REPO,
                check=True,
            )
        cls.txt = os.path.join(cls._tmp.name, "d.txt")
        # "abc" wird verworfen und erzeugt eine Warnung auf stderr.
        Path(cls.txt).write_text("1.5\nabc\n2.5\n3.1\n4.2\n2.2\n", encoding="utf-8")
        cls.validator = jsonschema.Draft202012Validator(SCHEMA)

    @classmethod
    def tearDownClass(cls):
        cls._tmp.cleanup()

    def raw_stdout(self, *args):
        return subprocess.run(
            [self.binary, *args], capture_output=True, text=True
        )

    def test_stdout_validates_against_schema(self):
        result = self.raw_stdout("-file=" + WINE, "-column=density", "-all", "-format=json")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.validator.validate(json.loads(result.stdout))

    def test_client_reads_report(self):
        report = run_distfit(self.binary, ["-file=" + WINE, "-column=density", "-all", "-format=json"])
        self.assertIsInstance(report, FitReport)
        self.assertEqual(report.schema_version, 1)
        self.assertEqual(report.sample_size, 1599)
        self.assertEqual(report.column, "density")
        self.assertEqual(report.best_distribution, report.models[0].name)
        self.assertEqual(report.models[0].delta_aic, 0)
        self.assertIsInstance(report.failures, list)

    def test_warnings_on_stderr_keep_stdout_parseable(self):
        result = self.raw_stdout("-file=" + self.txt, "-all", "-format=json")
        self.assertEqual(result.returncode, 0)
        self.assertIn("Warnung", result.stderr)
        report = FitReport.from_json(result.stdout)
        self.assertEqual(report.discarded, 1)
        self.assertIsNone(report.column)

    def test_error_gives_exit_code_and_empty_stdout(self):
        result = self.raw_stdout("-file=does-not-exist.txt", "-all", "-format=json")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout, "")
        self.assertTrue(result.stderr.strip())

    def test_schema_rejects_missing_schema_version(self):
        doc = json.loads(self.raw_stdout("-file=" + WINE, "-column=density", "-all", "-format=json").stdout)
        del doc["schema_version"]
        with self.assertRaises(jsonschema.ValidationError):
            self.validator.validate(doc)

    def test_client_ignores_additional_fields(self):
        doc = json.loads(self.raw_stdout("-file=" + WINE, "-column=density", "-all", "-format=json").stdout)
        doc["future_field"] = {"anything": True}
        doc["models"][0]["future_metric"] = 1.0
        report = FitReport.from_json(json.dumps(doc))
        self.assertEqual(report.models[0].name, doc["models"][0]["name"])

    def test_client_rejects_unknown_schema_version(self):
        doc = json.loads(self.raw_stdout("-file=" + WINE, "-column=density", "-all", "-format=json").stdout)
        doc["schema_version"] = 2
        with self.assertRaises(ValueError):
            FitReport.from_json(json.dumps(doc))


if __name__ == "__main__":
    unittest.main()
