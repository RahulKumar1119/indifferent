import { AwsRum, type AwsRumConfig } from 'aws-rum-web';

// CloudWatch RUM - Real User Monitoring
try {
  const config: AwsRumConfig = {
    sessionSampleRate: 1,
    identityPoolId: 'ap-south-1:6a7f63ba-3e57-4ff8-a7d7-296f9a49b880',
    endpoint: 'https://dataplane.rum.ap-south-1.amazonaws.com',
    telemetries: ['performance', 'errors', 'http'],
    allowCookies: true,
    enableXRay: true,
    signing: true,
  };

  new AwsRum('15e33481-cc75-479b-aff9-6781ccead8f6', '1.0.0', 'ap-south-1', config);
} catch (error) {
  // Ignore errors thrown during CloudWatch RUM initialization
}

import { bootstrapApplication } from '@angular/platform-browser';
import { appConfig } from './app/app.config';
import { App } from './app/app';

bootstrapApplication(App, appConfig)
  .catch((err) => console.error(err));
