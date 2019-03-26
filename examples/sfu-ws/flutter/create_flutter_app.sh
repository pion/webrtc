#/!/bin/bash
flutter create --project-name flutter_pions_sfu_ws .
cp app_config/AndroidManifest.xml android/app/src/main/
cp app_config/Info.plist ios/Runner/
cp app_config/Podfile ios/
