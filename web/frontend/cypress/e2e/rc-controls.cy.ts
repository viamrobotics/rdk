describe('should load the page', () => {
  it('passes', () => {
    cy.visit('http://127.0.0.1:8080/');

    cy.contains('h2', 'test_base').should('exist');
    cy.contains('h2', 'test_gantry').should('exist');
    cy.contains('h2', 'test_movement').should('exist');
    cy.contains('h2', 'test_arm').should('exist');
    cy.contains('h2', 'test_gripper').should('exist');
    cy.contains('h2', 'test_servo').should('exist');
    cy.contains('h2', 'test_motor').should('exist');
    cy.contains('h2', 'test_input').should('exist');
    cy.contains('h2', "WebGamepad").should('exist');
    cy.contains('h2', 'test_board').should('exist');
    cy.contains('h2', 'test_camera').should('exist');
    cy.contains('h2', 'Sensors').should('exist');
    cy.contains('h2', 'Navigation Service').should('exist');
    cy.contains('h2', 'Current Operations').should('exist');
    cy.contains('h2', 'Do()').should('exist');
  })
})