import { HeadlessPage } from './headless-page'
import type { HeadlessPageProps } from './headless-client'

export type MeasurePageProps = HeadlessPageProps

export function MeasurePage(props: MeasurePageProps) {
  return <HeadlessPage {...props} kind="measure" />
}
