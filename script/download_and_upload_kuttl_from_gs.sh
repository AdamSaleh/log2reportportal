cd  kuttl-$NAME
gsutil -m ls gs://origin-ci-test/logs/periodic-ci-redhat-developer-gitops-operator-master-v4.12-periodic-kuttl-$NAME/ | awk -F  "/" '{print $6}' | sort | tail -n10 |  xargs -I '{}' mkdir -p {}
find . -type d -empty -exec echo {} \; | awk -F  "/" '{print $2}' | grep -v latest | xargs -I {} gsutil -m cp -r gs://origin-ci-test/logs/periodic-ci-redhat-developer-gitops-operator-master-v4.12-periodic-kuttl-$NAME/{}/artifacts/periodic-kuttl-$NAME/$NAME-e2e-steps/ {}
find . -type d -empty -print -delete
cd ..
ls kuttl-$NAME | grep -v latest | xargs -P 16 -I '{}' ../log2reportportal -launch {} -name $NAME-kuttl -project gitops-nightly -skipExisting -skipTls -file kuttl-$NAME/{}/$NAME-e2e-steps/build-log.txt 
