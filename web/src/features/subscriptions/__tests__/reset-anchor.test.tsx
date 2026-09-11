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

import { fireEvent, render, screen } from '@testing-library/react'
import type { TFunction } from 'i18next'
import { useForm } from 'react-hook-form'
import { describe, expect, it, vi } from 'vitest'

import { Form } from '@/components/ui/form'

import { SubscriptionResetAnchorField } from '../components/subscription-reset-anchor-field'
import {
  PLAN_FORM_DEFAULTS,
  formValuesToPlanPayload,
  getPlanFormSchema,
  planToFormValues,
  type PlanFormValues,
} from '../lib/plan-form'
import type { SubscriptionPlan } from '../types'

function Harness(props: {
  period?: PlanFormValues['quota_reset_period']
  isEdit?: boolean
  anchor?: number
  onSubmit?: (values: PlanFormValues) => void
}) {
  const form = useForm<PlanFormValues>({
    defaultValues: {
      ...PLAN_FORM_DEFAULTS,
      quota_reset_period: props.period || 'custom',
      quota_reset_anchor_time: props.anchor || 0,
    },
  })
  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit((v) => props.onSubmit?.(v))}>
        <SubscriptionResetAnchorField isEdit={props.isEdit || false} />
        <button type='submit'>Save</button>
      </form>
    </Form>
  )
}

describe('subscription reset anchor', () => {
  it('allows a new custom plan to change time and submit an absolute timestamp', async () => {
    const anchor = new Date(2026, 8, 15, 10, 2, 0)
    const save = vi.fn()
    render(<Harness anchor={anchor.getTime() / 1000} onSubmit={save} />)
    fireEvent.change(screen.getByDisplayValue('10:02'), {
      target: { value: '11:03' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await vi.waitFor(() => expect(save).toHaveBeenCalled())
    expect(save.mock.calls[0][0].quota_reset_anchor_time).toBe(
      new Date(2026, 8, 15, 11, 3, 0).getTime() / 1000
    )
  })
  it('clearing the date restores activation-based resets', async () => {
    const save = vi.fn()
    render(<Harness anchor={1789437720} onSubmit={save} />)
    fireEvent.click(screen.getByRole('button', { name: 'Clear' }))
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await vi.waitFor(() => expect(save).toHaveBeenCalled())
    expect(save.mock.calls[0][0].quota_reset_anchor_time).toBe(0)
  })
  it('does not show an anchor picker for calendar reset cycles', () => {
    render(<Harness period='weekly' />)
    expect(screen.queryByRole('group')).not.toBeInTheDocument()
  })
  it('shows an existing anchor read-only when editing a plan', () => {
    render(<Harness isEdit anchor={1789437720} />)
    expect(
      screen.getByText('The reset anchor can only be set when creating a plan.')
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Clear' })
    ).not.toBeInTheDocument()
    expect(screen.getAllByRole('button')).toHaveLength(1)
  })
  it('omits a previously chosen anchor when switching away from a custom cycle', () => {
    const values = {
      ...PLAN_FORM_DEFAULTS,
      quota_reset_anchor_time: 1789437720,
      quota_reset_period: 'weekly' as const,
    }
    expect(formValuesToPlanPayload(values).plan.quota_reset_anchor_time).toBe(0)
  })
  it('preserves saved anchors in form round-trips and defaults legacy plans to zero', () => {
    const plan = {
      ...PLAN_FORM_DEFAULTS,
      id: 1,
      currency: 'USD',
      quota_reset_period: 'custom',
      quota_reset_anchor_time: 1789437720,
    } as SubscriptionPlan
    expect(
      formValuesToPlanPayload(planToFormValues(plan)).plan
        .quota_reset_anchor_time
    ).toBe(1789437720)
    expect(
      planToFormValues({ ...plan, quota_reset_anchor_time: undefined })
        .quota_reset_anchor_time
    ).toBe(0)
  })
  it('rejects invalid or out-of-range timestamps', () => {
    const schema = getPlanFormSchema(((s: string) => s) as TFunction)
    for (const quota_reset_anchor_time of [-1, Number.NaN, 253402300800]) {
      expect(
        schema.safeParse({
          ...PLAN_FORM_DEFAULTS,
          title: 'Test',
          quota_reset_anchor_time,
        }).success
      ).toBe(false)
    }
  })
})
