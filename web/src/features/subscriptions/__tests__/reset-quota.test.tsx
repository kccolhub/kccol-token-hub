/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { ResetSubscriptionsDialog } from '../components/dialogs/reset-subscriptions-dialog'
import {
  SubscriptionsProvider,
  useSubscriptions,
} from '../components/subscriptions-provider'
import { PLAN_FORM_DEFAULTS } from '../lib/plan-form'
import type { SubscriptionPlan } from '../types'

function Harness(props: { period?: string }) {
  const context = useSubscriptions()
  return (
    <>
      <button
        type='button'
        onClick={() => {
          context.setCurrentRow({
            plan: {
              ...PLAN_FORM_DEFAULTS,
              id: 101,
              currency: 'USD',
              title: 'Shared plan',
              quota_reset_period: props.period || 'custom',
              quota_reset_custom_seconds: 604800,
            } as SubscriptionPlan,
          })
          context.setOpen('reset-subscriptions')
        }}
      >
        Open reset
      </button>
      <ResetSubscriptionsDialog />
    </>
  )
}

function renderReset(period?: string) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  client.setQueryData(['system-options'], { success: true, data: [] })
  return render(
    <QueryClientProvider client={client}>
      <SubscriptionsProvider>
        <Harness period={period} />
      </SubscriptionsProvider>
    </QueryClientProvider>
  )
}

afterEach(() => vi.restoreAllMocks())

it('restarting now also enables advancing time and submits the shortcut to the plan reset API', async () => {
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValue({ data: { success: true, data: { reset_count: 2 } } })
  renderReset()
  fireEvent.click(screen.getByRole('button', { name: 'Open reset' }))
  const restart = screen.getByRole('switch', { name: 'Start a new cycle now' })
  expect(restart).not.toBeChecked()
  fireEvent.click(
    screen.getByRole('switch', { name: 'Advance next reset time' })
  )
  fireEvent.click(restart)
  expect(
    screen.getByRole('switch', { name: 'Advance next reset time' })
  ).toBeChecked()
  fireEvent.click(screen.getByRole('button', { name: 'Reset quota' }))
  await waitFor(() =>
    expect(post).toHaveBeenCalledWith(
      '/api/subscription/admin/plans/101/subscriptions/reset',
      { advance_reset_time: true, restart_cycle_now: true }
    )
  )
  await waitFor(() =>
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  )
})

it('keeping reset times cancels the restart shortcut and reopening restores defaults', async () => {
  renderReset()
  fireEvent.click(screen.getByRole('button', { name: 'Open reset' }))
  fireEvent.click(screen.getByRole('switch', { name: 'Start a new cycle now' }))
  fireEvent.click(
    screen.getByRole('switch', { name: 'Advance next reset time' })
  )
  expect(
    screen.getByRole('switch', { name: 'Start a new cycle now' })
  ).not.toBeChecked()
  fireEvent.click(screen.getByRole('switch', { name: 'Start a new cycle now' }))
  fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
  await waitFor(() =>
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  )
  fireEvent.click(screen.getByRole('button', { name: 'Open reset' }))
  expect(
    screen.getByRole('switch', { name: 'Start a new cycle now' })
  ).not.toBeChecked()
})

it('does not offer restarting a custom cycle for a calendar plan', () => {
  renderReset('weekly')
  fireEvent.click(screen.getByRole('button', { name: 'Open reset' }))
  expect(
    screen.queryByRole('switch', { name: 'Start a new cycle now' })
  ).not.toBeInTheDocument()
})

it('keeps the shortcut selected after a failed request so the administrator can retry', async () => {
  const post = vi
    .spyOn(api, 'post')
    .mockRejectedValueOnce(new Error('offline'))
    .mockResolvedValueOnce({
      data: { success: true, data: { reset_count: 2 } },
    })
  renderReset()
  fireEvent.click(screen.getByRole('button', { name: 'Open reset' }))
  fireEvent.click(screen.getByRole('switch', { name: 'Start a new cycle now' }))
  fireEvent.click(screen.getByRole('button', { name: 'Reset quota' }))
  await waitFor(() => expect(post).toHaveBeenCalledTimes(1))
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Reset quota' })).toBeEnabled()
  )
  expect(
    screen.getByRole('switch', { name: 'Start a new cycle now' })
  ).toBeChecked()
  fireEvent.click(screen.getByRole('button', { name: 'Reset quota' }))
  await waitFor(() =>
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  )
  expect(post).toHaveBeenCalledTimes(2)
})
