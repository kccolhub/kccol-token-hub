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
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { JsonCodeEditor } from '@/components/json-code-editor'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const limitSchema = z.number().int().min(0).max(100000)
const groupSchema = z.record(
  z
    .string()
    .min(1)
    .refine((key) => key.trim() === key),
  limitSchema
)
const userSchema = z.record(
  z
    .string()
    .regex(/^[1-9][0-9]*$/)
    .refine((key) => Number.isSafeInteger(Number(key))),
  limitSchema
)
const configSchema = z.object({
  enabled: z.boolean().default(false),
  default_limit: limitSchema.default(0),
  groups: groupSchema.nullish().transform((value) => value ?? {}),
  users: userSchema.nullish().transform((value) => value ?? {}),
})

function validOverrides(value: string, users: boolean): boolean {
  try {
    return (users ? userSchema : groupSchema).safeParse(JSON.parse(value))
      .success
  } catch {
    return false
  }
}

function formDefaults(value: string) {
  const cfg = configSchema.parse(JSON.parse(value))
  return {
    ...cfg,
    groups: JSON.stringify(cfg.groups, null, 2),
    users: JSON.stringify(cfg.users, null, 2),
  }
}

type ConcurrencySectionProps = { value: string }

export function ConcurrencySection(props: ConcurrencySectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const schema = z.object({
    enabled: z.boolean(),
    default_limit: limitSchema,
    groups: z
      .string()
      .refine(
        (value) => validOverrides(value, false),
        t(
          'Use a JSON object with group names and integer limits from 0 to 100000.'
        )
      ),
    users: z
      .string()
      .refine(
        (value) => validOverrides(value, true),
        t(
          'Use a JSON object with positive user IDs and integer limits from 0 to 100000.'
        )
      ),
  })
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    mode: 'onChange',
    defaultValues: formDefaults(props.value),
  })
  useEffect(() => {
    form.reset(formDefaults(props.value))
  }, [props.value, form])

  const onSubmit = async (values: z.infer<typeof schema>) => {
    try {
      await updateOption.mutateAsync({
        key: 'UserConcurrencyLimit',
        value: JSON.stringify({
          ...values,
          groups: JSON.parse(values.groups),
          users: JSON.parse(values.users),
        }),
      })
    } catch {
      // The shared mutation reports the error; keep the draft for retry.
    }
  }

  return (
    <SettingsSection title={t('User Concurrency')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save concurrency limits'
          />
          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable user concurrency limits')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Count active requests across all API keys of the same user. Excess requests return 429. Streams occupy a slot until they finish; asynchronous jobs occupy a slot only during submission.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />
          <FormField
            control={form.control}
            name='default_limit'
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  {t('Default concurrent requests per user')}
                </FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={0}
                    max={100000}
                    step={1}
                    {...field}
                    onChange={(event) =>
                      field.onChange(
                        event.target.value === ''
                          ? Number.NaN
                          : Number(event.target.value)
                      )
                    }
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    '0 means unlimited. User overrides take priority over the user group, then the default. Changes apply to new requests.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='groups'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Concurrency by user group')}</FormLabel>
                <FormControl>
                  <JsonCodeEditor
                    value={field.value}
                    onChange={field.onChange}
                    name={field.name}
                    onBlur={field.onBlur}
                    textareaRef={field.ref}
                    placeholder='{"default": 3, "vip": 10}'
                    aria-invalid={Boolean(form.formState.errors.groups)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Uses the account group, independently of the group selected by an API key. Example: {"default": 3, "vip": 10}.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='users'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Concurrency by user ID')}</FormLabel>
                <FormControl>
                  <JsonCodeEditor
                    value={field.value}
                    onChange={field.onChange}
                    name={field.name}
                    onBlur={field.onBlur}
                    textareaRef={field.ref}
                    placeholder='{"123": 5}'
                    aria-invalid={Boolean(form.formState.errors.users)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Example: {"123": 5}. Remove an entry to inherit its group limit; 0 explicitly allows unlimited requests.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
