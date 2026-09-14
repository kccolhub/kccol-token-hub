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
import { useFormContext, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { DateTimePicker } from '@/components/datetime-picker'
import {
  FormDescription,
  FormField,
  FormItem,
  FormMessage,
} from '@/components/ui/form'

import type { PlanFormValues } from '../lib/plan-form'

export function SubscriptionResetAnchorField(props: { isEdit: boolean }) {
  const { t } = useTranslation()
  const form = useFormContext<PlanFormValues>()
  const period = useWatch({ control: form.control, name: 'quota_reset_period' })
  const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone

  if (period !== 'custom') return null

  return (
    <FormField
      control={form.control}
      name='quota_reset_anchor_time'
      render={({ field }) => (
        <FormItem>
          <fieldset className='space-y-2'>
            <legend className='text-sm font-medium'>
              {t('Reset anchor (optional)')}
            </legend>
            <DateTimePicker
              value={field.value ? new Date(field.value * 1000) : undefined}
              onChange={(date) =>
                field.onChange(date ? Math.floor(date.getTime() / 1000) : 0)
              }
              placeholder={
                props.isEdit
                  ? t('Start a new interval when saving')
                  : t('Start from subscription activation')
              }
            />
            <FormDescription>
              {t(
                props.isEdit
                  ? 'Time zone: {{timezone}}. Active subscriptions follow this anchor and interval. Past anchors advance to the next future boundary. Clearing the anchor starts a new interval when you save.'
                  : 'Time zone: {{timezone}}. All subscriptions follow this anchor and interval. Past anchors advance to the next future boundary. Leave empty to start from activation.',
                { timezone }
              )}
            </FormDescription>
            <FormMessage />
          </fieldset>
        </FormItem>
      )}
    />
  )
}
