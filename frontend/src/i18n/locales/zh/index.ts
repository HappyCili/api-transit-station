import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import imageGeneration from './imageGeneration'
import admin from './admin'
import misc from './misc'

export default {
  ...landing,
  ...common,
  ...dashboard,
  ...imageGeneration,
  admin,
  ...misc,
}
