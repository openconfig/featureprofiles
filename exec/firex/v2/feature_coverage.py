from firexapp.engine.celery import app
from microservices.firex_base import InjectArgs
from microservices.chaintasks import RunTests
from microservices.feature_coverage import CerebroGetTestsForFeature
from diff2func import map_file_to_pims_comp
import os

@app.task(bind=True)
def B4FeatureCoverageRunTests(self, uid, feature_id, platforms=["8000"], testsuites=None):
    get_tests_result = self.enqueue_child_and_get_results(
        CerebroGetTestsForFeature.s(uid=uid, feature_id=feature_id, platforms=platforms))

    cerebro_feature_files = get_tests_result['cerebro_feature_files']
    if not cerebro_feature_files:
        raise Exception(
            f"Cerebro has no files for {feature_id}. Coverage can't be found without "
            "files to instrument. Make sure all DDTSes are tagged with this feature "
            "and have diffs."
        )

    run_tests_results = self.enqueue_child_and_get_results(
        InjectArgs(**self.abog)
        | RunTests.s(
            testsuites=testsuites,
            files=cerebro_feature_files,
            cflow=True,
            # CflowList requires comps, ignores files otherwise :/
            comps=sorted({map_file_to_pims_comp(f) for f in cerebro_feature_files}),
            # CflowList has a weird contract where it requires comps
            # in order to process files, but then it assumes you
            # want to instrument comps instead of the files.
            instrument_comps=False,
            cov_testtype='dt',
            # Send data to Cerebro for DT reporting/dashboards,
            # but do not populate smart sanity DB
            cerebro_export=False,
            cerebro_smart_sanity_export=False,
            # fail if converted TB file used for coverage
            # collection has no devices, since this guarantees
            # no coverage.
            require_converted_tb_devices=True,
        )
    )
    coverage_xml_file = run_tests_results['coverage_xml_file']
    assert coverage_xml_file and os.path.isfile(coverage_xml_file), \
        f'Failed to generate coverage XML file: {coverage_xml_file}'