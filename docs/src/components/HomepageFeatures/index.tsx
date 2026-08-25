import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import Heading from '@theme/Heading';
import styles from './styles.module.css';

type FeatureItem = {
  title: string;
  to: string;
  description: ReactNode;
};

const FeatureList: FeatureItem[] = [
  {
    title: 'FlowBalanceException (E1)',
    to: '/docs/business-context/ubiquitous-language',
    description: (
      <>
        Correlates a wes rebalance recommendation, a workforce-management
        staffing gap, and a fulfillment-execution stuck-task diagnostic into
        one ranked <code>assign_labor</code> / <code>release_next_work</code>{' '}
        / <code>hold</code> recommendation.
      </>
    ),
  },
  {
    title: 'StrandedReservation (E2)',
    to: '/docs/business-context/ubiquitous-language',
    description: (
      <>
        Correlates expired task leases with a usable-stock shortfall into a{' '}
        <code>revoke_reservation</code> recommendation — never returned
        without its mandatory blast-radius readout.
      </>
    ),
  },
  {
    title: 'DailyBrief (E3)',
    to: '/docs/business-context/ubiquitous-language',
    description: (
      <>
        Synthesizes every monitored path across every site into one brief,
        flagging an open exception only when two or more independent
        signals correlate — never a single metric alone.
      </>
    ),
  },
  {
    title: 'Recommendations-only, v1',
    to: '/docs/mcp/governance-note',
    description: (
      <>
        Reads everywhere via five Open Host Services, writes nowhere. Every
        decision carries its full evidence trail and degrades to a
        conservative <code>hold</code> when a signal is missing.
      </>
    ),
  },
];

function Feature({title, to, description}: FeatureItem) {
  return (
    <div className={clsx('col col--3')}>
      <div className={styles.featureCard}>
        <Heading as="h3" className={styles.featureTitle}>
          <Link to={to}>{title}</Link>
        </Heading>
        <p className={styles.featureBody}>{description}</p>
      </div>
    </div>
  );
}

export default function HomepageFeatures(): ReactNode {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}
