import { useState, useEffect } from 'react';
import {
  Box,
  TextField,
  FormControl,
  FormControlLabel,
  Switch,
  Select,
  MenuItem,
  InputLabel,
  Typography,
  Divider,
} from '@mui/material';

interface ConfigSchema {
  type: 'object';
  properties: Record<string, {
    type: 'string' | 'number' | 'boolean' | 'enum';
    title?: string;
    description?: string;
    default?: any;
    enum?: string[];
    minimum?: number;
    maximum?: number;
    pattern?: string;
  }>;
  required?: string[];
}

interface PluginConfigFormProps {
  schema?: ConfigSchema;
  value: Record<string, any>;
  onChange: (value: Record<string, any>) => void;
  disabled?: boolean;
}

/**
 * PluginConfigForm - Dynamic form generator for plugin configuration
 *
 * Generates a configuration form based on a JSON schema definition. Supports
 * various field types (string, number, boolean, enum) with validation, required
 * fields, and helper text. Automatically creates appropriate UI components
 * based on field types.
 *
 * Features:
 * - Dynamic form generation from JSON schema
 * - Support for string, number, boolean, and enum fields
 * - Required field indicators
 * - Field descriptions and helper text
 * - Validation (pattern, min/max for numbers)
 * - Real-time value updates via onChange callback
 * - Disabled state support
 *
 * @component
 *
 * @param {Object} props - Component props
 * @param {ConfigSchema} [props.schema] - JSON schema defining form structure
 * @param {Record<string, any>} props.value - Current form values
 * @param {Function} props.onChange - Callback when form values change
 * @param {boolean} [props.disabled=false] - Whether form fields are disabled
 *
 * @returns {JSX.Element} Rendered configuration form
 *
 * @example
 * <PluginConfigForm
 *   schema={pluginManifest.configSchema}
 *   value={config}
 *   onChange={setConfig}
 * />
 *
 * @example
 * // Schema example
 * const schema = {
 *   type: 'object',
 *   properties: {
 *     apiKey: { type: 'string', title: 'API Key', description: 'Your API key' },
 *     enabled: { type: 'boolean', title: 'Enable Plugin' },
 *     mode: { type: 'enum', enum: ['dev', 'prod'] }
 *   },
 *   required: ['apiKey']
 * }
 */
export default function PluginConfigForm({
  schema,
  value,
  onChange,
  disabled = false,
}: PluginConfigFormProps) {
  const [formData, setFormData] = useState<Record<string, any>>(value || {});

  useEffect(() => {
    setFormData(value || {});
  }, [value]);

  const handleFieldChange = (fieldName: string, fieldValue: any) => {
    const newData = { ...formData, [fieldName]: fieldValue };
    setFormData(newData);
    onChange(newData);
  };

  if (!schema || !schema.properties || Object.keys(schema.properties).length === 0) {
    return (
      <Box>
        <Typography variant="body2" color="text.secondary" mb={2}>
          No configuration schema available. Use JSON editor below.
        </Typography>
      </Box>
    );
  }

  const renderField = (fieldName: string, fieldSchema: any) => {
    const fieldTitle = fieldSchema.title || fieldName;
    const fieldDescription = fieldSchema.description;
    const isRequired = schema.required?.includes(fieldName);
    const fieldValue = formData[fieldName] ?? fieldSchema.default;

    switch (fieldSchema.type) {
      case 'boolean':
        return (
          <FormControlLabel
            key={fieldName}
            control={
              <Switch
                checked={fieldValue || false}
                onChange={(e) => handleFieldChange(fieldName, e.target.checked)}
                disabled={disabled}
              />
            }
            label={
              <Box>
                <Typography variant="body2">
                  {fieldTitle}
                  {isRequired && <span style={{ color: 'red' }}> *</span>}
                </Typography>
                {fieldDescription && (
                  <Typography variant="caption" color="text.secondary">
                    {fieldDescription}
                  </Typography>
                )}
              </Box>
            }
          />
        );

      case 'enum':
        return (
          <FormControl key={fieldName} fullWidth>
            <InputLabel>
              {fieldTitle}
              {isRequired && <span style={{ color: 'red' }}> *</span>}
            </InputLabel>
            <Select
              value={fieldValue || ''}
              onChange={(e) => handleFieldChange(fieldName, e.target.value)}
              label={fieldTitle}
              disabled={disabled}
            >
              {fieldSchema.enum?.map((option: string) => (
                <MenuItem key={option} value={option}>
                  {option}
                </MenuItem>
              ))}
            </Select>
            {fieldDescription && (
              <Typography variant="caption" color="text.secondary" mt={0.5}>
                {fieldDescription}
              </Typography>
            )}
          </FormControl>
        );

      case 'number':
        return (
          <TextField
            key={fieldName}
            fullWidth
            type="number"
            label={fieldTitle}
            value={fieldValue ?? ''}
            onChange={(e) => handleFieldChange(fieldName, parseFloat(e.target.value) || 0)}
            disabled={disabled}
            required={isRequired}
            helperText={fieldDescription}
            inputProps={{
              min: fieldSchema.minimum,
              max: fieldSchema.maximum,
            }}
          />
        );

      case 'string':
      default:
        return (
          <TextField
            key={fieldName}
            fullWidth
            label={fieldTitle}
            value={fieldValue || ''}
            onChange={(e) => handleFieldChange(fieldName, e.target.value)}
            disabled={disabled}
            required={isRequired}
            helperText={fieldDescription}
            inputProps={{
              pattern: fieldSchema.pattern,
            }}
          />
        );
    }
  };

  return (
    <Box>
      <Typography variant="subtitle2" gutterBottom>
        Plugin Configuration
      </Typography>
      <Typography variant="caption" color="text.secondary" display="block" mb={2}>
        Configure plugin settings using the form below.
      </Typography>

      <Box display="flex" flexDirection="column" gap={2}>
        {Object.entries(schema.properties).map(([fieldName, fieldSchema]) =>
          renderField(fieldName, fieldSchema)
        )}
      </Box>

      <Divider sx={{ my: 2 }} />

      <Typography variant="caption" color="text.secondary">
        Fields marked with <span style={{ color: 'red' }}>*</span> are required
      </Typography>
    </Box>
  );
}
