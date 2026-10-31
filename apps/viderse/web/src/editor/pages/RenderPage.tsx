import { HeadlessPage } from './headless-page'
import type { HeadlessPageProps } from './headless-client'

export type RenderPageProps = HeadlessPageProps

export function RenderPage(props: RenderPageProps) {
  return <HeadlessPage {...props} kind="render" />
}
