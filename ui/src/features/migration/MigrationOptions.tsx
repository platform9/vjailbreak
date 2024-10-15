import { useCallback } from "react"
import {
  Checkbox,
  FormControlLabel,
  MenuItem,
  Select,
  styled,
  TextField,
} from "@mui/material"
import dayjs from "dayjs"
import { DateTimePicker } from "@mui/x-date-pickers/DateTimePicker"
import { LocalizationProvider } from "@mui/x-date-pickers/LocalizationProvider"
import { AdapterDayjs } from "@mui/x-date-pickers/AdapterDayjs"
import Step from "src/components/forms/Step"
import { FormValues, MigrationOptionsType, Errors } from "./MigrationForm"

// Accordian Imports
import Accordion from "@mui/material/Accordion"
import AccordionSummary from "@mui/material/AccordionSummary"
import AccordionDetails from "@mui/material/AccordionDetails"
import ExpandMoreIcon from "@mui/icons-material/ExpandMore"

// Styles
const FieldsContainer = styled("div")(({ theme }) => ({
  marginLeft: theme.spacing(4),
}))

const Fields = styled("div")(({ theme }) => ({
  display: "grid",
  gridTemplateColumns: "1fr 2fr 1fr",
  gridGap: "16px", // Adds spacing between the columns
  marginTop: theme.spacing(2),
}))

const CustomTextField = styled(TextField)({
  "& .MuiOutlinedInput-root": {
    fontFamily: "Monospace",
  },
})

// Intefaces
interface MigrationOptionsPropsInterface {
  params: FormValues
  onChange: (key: string) => (value: unknown) => void
  migrationOptions: MigrationOptionsType
  updateMigrationOptions: (
    key: keyof MigrationOptionsType
  ) => (value: unknown) => void
  errors: Errors
  getErrorsUpdater: (key: string | number) => (value: string) => void
}

// Constants
const DATA_COPY_METHODS = [
  { value: "hot", label: "Copy live VMs then power off" },
  { value: "cold", label: "Power off live VMs then copy" },
]

const VM_CUTOVER_OPTIONS = [
  { value: "0", label: "Cutover immediately after data copy" },
  { value: "1", label: "Admin initiated cutover" },
  { value: "2", label: "Cutover during time window" },
]

// TODO - Commented out the non-required field from the options for now
// const PrePostWebHooksList = [
//   { label: "Pre data-copy web hook", identifier: "preDataCopyWebHook" },
//   { label: "Post data-copy web hook", identifier: "postDataCopyWebHook" },
//   { label: "Pre cutover web hook", identifier: "preCutoverWebHook" },
//   { label: "Post cutover web hook", identifier: "postCutoverWebHook" },
// ]

export default function MigrationOptions({
  params,
  onChange,
  migrationOptions,
  updateMigrationOptions,
  errors,
  getErrorsUpdater,
}: MigrationOptionsPropsInterface) {
  // Validate Required fields

  return (
    <LocalizationProvider dateAdapter={AdapterDayjs}>
      <Accordion
        sx={{
          boxShadow: "none", // Removes box shadow
          border: "none", // Removes border
          "&:before": {
            display: "none", // Removes the default divider line before the accordion
          },
        }}
        defaultExpanded
      >
        <AccordionSummary
          expandIcon={<ExpandMoreIcon />}
          aria-controls="panel2-content"
          id="panel2-header"
        >
          <Step
            stepNumber="4"
            label="Migration Options (Optional)"
            sx={{ mb: "0" }}
          />
        </AccordionSummary>
        <AccordionDetails>
          <FieldsContainer>
            {/* Retry on failure */}
            <FormControlLabel
              label="Retry on failure"
              control={
                <Checkbox
                  checked={params?.retryOnFailure || false}
                  onChange={(e) => {
                    onChange("retryOnFailure")(e.target.checked)
                  }}
                />
              }
            />

            {/* Data Copy */}
            <Fields>
              <FormControlLabel
                id="data-copy-method"
                label="Data copy method"
                control={
                  <Checkbox
                    checked={migrationOptions.dataCopyMethod}
                    onChange={(e) => {
                      updateMigrationOptions("dataCopyMethod")(e.target.checked)
                    }}
                  />
                }
              />
              <Select
                size="small"
                disabled={!migrationOptions.dataCopyMethod}
                labelId="source-item-label"
                value={params?.dataCopyMethod || "hot"}
                onChange={(e) => {
                  onChange("dataCopyMethod")(e.target.value)
                  updateMigrationOptions("dataCopyMethod")(true)
                }}
              >
                {DATA_COPY_METHODS.map((item) => (
                  <MenuItem key={item.value} value={item.value}>
                    {item.label}
                  </MenuItem>
                ))}
              </Select>
            </Fields>

            {/* Data Copy Time Window */}
            <Fields>
              <FormControlLabel
                label={"Data copy start time"}
                control={
                  <Checkbox
                    checked={migrationOptions?.dataCopyStartTime}
                    onChange={(e) => {
                      updateMigrationOptions("dataCopyStartTime")(
                        e.target.checked
                      )
                    }}
                  />
                }
              />
              <TimePicker
                label="Start Time"
                identifier="dataCopyStartTime"
                params={params}
                errors={errors}
                onChange={onChange}
                disabled={!migrationOptions?.dataCopyStartTime}
                required={!!migrationOptions?.dataCopyStartTime}
              />
            </Fields>

            {/* Cutover settings*/}
            <Fields>
              <FormControlLabel
                id="data-copy-method"
                label="Cutover Options"
                control={
                  <Checkbox
                    checked={migrationOptions.cutoverOption}
                    onChange={(e) => {
                      updateMigrationOptions("cutoverOption")(e.target.checked)
                      onChange("cutoverOption")("0")
                    }}
                  />
                }
              />
              <Select
                size="small"
                disabled={!migrationOptions?.cutoverOption}
                value={params?.cutoverOption || "0"}
                onChange={(e) => {
                  onChange("cutoverOption")(e.target.value)
                  onChange("cutoverStartTime")(null)
                  onChange("cutoverEndTime")(null)
                }}
              >
                {VM_CUTOVER_OPTIONS.map((item) => (
                  <MenuItem key={item.value} value={item.value}>
                    {item.label}
                  </MenuItem>
                ))}
              </Select>
            </Fields>

            {params.cutoverOption === "2" && (
              <Fields sx={{ mt: "20px", gridTemplateColumns: "1fr 1fr 1fr" }}>
                <TimePicker
                  label="Start Time"
                  identifier="cutoverStartTime"
                  params={params}
                  errors={errors}
                  onChange={onChange}
                  sx={{ ml: "32px" }}
                  required={params.cutoverOption === "2"}
                />
                <TimePicker
                  label="End Time"
                  identifier="cutoverEndTime"
                  params={params}
                  errors={errors}
                  onChange={onChange}
                  required={params.cutoverOption === "2"}
                />
              </Fields>
            )}

            <Fields>
              <FormControlLabel
                label="Post migration script"
                control={
                  <Checkbox
                    checked={migrationOptions.postMigrationScript}
                    onChange={(e) => {
                      updateMigrationOptions("postMigrationScript")(
                        e.target.checked
                      )
                    }}
                  />
                }
              />
              <CustomTextField
                label="Post migration script"
                size="small"
                value={params?.postMigrationScript || ""}
                onChange={(e) =>
                  onChange("postMigrationScript")(String(e.target.value))
                }
                disabled={!migrationOptions.postMigrationScript}
                error={!!errors["postMigrationScript"]}
                required={migrationOptions.postMigrationScript}
              />
            </Fields>

            {/* Pre and Post Web Hooks */}
            {/* {PrePostWebHooksList.map((hook) => (
              <Fields key={`${hook.label}-${hook.identifier}`}>
                <PrePostWebHooks
                  label={hook.label}
                  identifier={hook.identifier}
                  params={params}
                  migrationOptions={migrationOptions}
                  updateMigrationOptions={updateMigrationOptions}
                  onChange={onChange}
                />
              </Fields>
            ))} */}
          </FieldsContainer>
        </AccordionDetails>
      </Accordion>
    </LocalizationProvider>
  )
}

const TimePicker = ({
  identifier,
  params,
  onChange,
  errors,
  required,
  ...props
}) => {
  const value = params?.[identifier] ? dayjs(params?.[identifier]) : null

  const handleTimeChange = useCallback(
    (newValue: dayjs.Dayjs | null, identifier) => {
      const formattedTime = newValue?.toISOString()
      onChange(identifier)(String(formattedTime))
    },
    [onChange]
  )

  return (
    <DateTimePicker
      ampm={false}
      value={value}
      onChange={(newValue: dayjs.Dayjs | null) =>
        handleTimeChange(newValue, identifier)
      }
      slots={{
        textField: (props) => (
          <TextField
            {...props}
            size="small"
            required={required}
            error={!!errors[identifier]} // Show error if validation fails
          />
        ),
      }}
      {...props}
    />
  )
}

// const PrePostWebHooks = ({
//   label,
//   identifier,
//   params,
//   onChange,
//   migrationOptions,
//   updateMigrationOptions,
// }) => {
//   return (
//     <>
//       <FormControlLabel
//         label={label}
//         control={
//           <Checkbox
//             checked={migrationOptions?.[identifier]}
//             onChange={(e) => {
//               updateMigrationOptions(identifier)(e.target.checked)
//             }}
//           />
//         }
//       />
//       <CustomTextField
//         value={params?.[identifier] || ""}
//         onChange={(e) => onChange(identifier)(String(e.target.value))}
//       />
//     </>
//   )
// }
